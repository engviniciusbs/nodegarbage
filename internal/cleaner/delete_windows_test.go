//go:build windows

package cleaner

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
)

func writeFile(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// lockFile opens path without FILE_SHARE_DELETE until the test ends.
func lockFile(t *testing.T, path string) {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { windows.CloseHandle(h) })
}

// victimTree builds root\victim\keep.txt and root\item\a.txt, with
// root\item\link a junction to victim. Returns item and keep.txt paths.
func victimTree(t *testing.T) (root, item, keep string) {
	t.Helper()
	root = t.TempDir()
	keep = filepath.Join(root, "victim", "keep.txt")
	writeFile(t, keep, 5)
	item = filepath.Join(root, "item")
	writeFile(t, filepath.Join(item, "a.txt"), 3)
	mkJunction(t, filepath.Join(item, "link"), filepath.Join(root, "victim"))
	return root, item, keep
}

func TestRemoveAllReadOnly(t *testing.T) {
	x := filepath.Join(t.TempDir(), "x")
	ro := filepath.Join(x, "sub", "ro.txt")
	writeFile(t, ro, 10)
	writeFile(t, filepath.Join(x, "b.txt"), 4)
	if err := os.Chmod(ro, 0o444); err != nil {
		t.Fatal(err)
	}
	freed, skipped, err := OSDeleter().RemoveAll(x)
	if err != nil || len(skipped) != 0 {
		t.Fatalf("err %v skipped %v", err, skipped)
	}
	if freed != 14 {
		t.Fatalf("freed = %d", freed)
	}
	if exists(x) {
		t.Fatal("x still exists")
	}
}

func TestRemoveAllSkipsLocked(t *testing.T) {
	x := filepath.Join(t.TempDir(), "x")
	locked := filepath.Join(x, "sub", "locked.txt")
	writeFile(t, locked, 8)
	writeFile(t, filepath.Join(x, "sub", "b.txt"), 2)
	writeFile(t, filepath.Join(x, "c.txt"), 3)
	lockFile(t, locked)

	freed, skipped, err := OSDeleter().RemoveAll(x)
	if err != nil {
		t.Fatal(err)
	}
	if freed != 5 {
		t.Fatalf("freed = %d", freed)
	}
	if len(skipped) != 1 || !strings.EqualFold(skipped[0], locked) {
		t.Fatalf("skipped = %v", skipped)
	}
	if !exists(locked) || exists(filepath.Join(x, "c.txt")) || exists(filepath.Join(x, "sub", "b.txt")) {
		t.Fatal("wrong files removed")
	}
	if !exists(x) {
		t.Fatal("root removed although it is not empty")
	}
}

func TestRemoveAllNothingRemoved(t *testing.T) {
	x := filepath.Join(t.TempDir(), "x")
	locked := filepath.Join(x, "locked.txt")
	writeFile(t, locked, 8)
	lockFile(t, locked)
	freed, skipped, err := OSDeleter().RemoveAll(x)
	if !errors.Is(err, ErrNothingRemoved) || freed != 0 || len(skipped) != 1 {
		t.Fatalf("freed %d skipped %v err %v", freed, skipped, err)
	}
}

func TestRemoveAllMissing(t *testing.T) {
	_, _, err := OSDeleter().RemoveAll(filepath.Join(t.TempDir(), "nope"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v", err)
	}
}

func TestRemoveContentsKeepsRoot(t *testing.T) {
	x := t.TempDir()
	writeFile(t, filepath.Join(x, "a.txt"), 6)
	writeFile(t, filepath.Join(x, "d", "e", "f.txt"), 1)
	freed, skipped, err := OSDeleter().RemoveContents(x)
	if err != nil || len(skipped) != 0 || freed != 7 {
		t.Fatalf("freed %d skipped %v err %v", freed, skipped, err)
	}
	ents, err := os.ReadDir(x)
	if err != nil {
		t.Fatal("root removed:", err)
	}
	if len(ents) != 0 {
		t.Fatalf("left %d entries", len(ents))
	}
}

func TestRemoveAllDoesNotFollowJunction(t *testing.T) {
	_, item, keep := victimTree(t)
	freed, skipped, err := OSDeleter().RemoveAll(item)
	if err != nil || len(skipped) != 0 {
		t.Fatalf("skipped %v err %v", skipped, err)
	}
	if freed != 3 {
		t.Fatalf("freed = %d (counted the junction target?)", freed)
	}
	if !exists(keep) {
		t.Fatal("junction target was deleted")
	}
	if exists(item) {
		t.Fatal("item still exists")
	}
}

func TestRemoveContentsDoesNotFollowJunction(t *testing.T) {
	_, item, keep := victimTree(t)
	freed, skipped, err := OSDeleter().RemoveContents(item)
	if err != nil || len(skipped) != 0 || freed != 3 {
		t.Fatalf("freed %d skipped %v err %v", freed, skipped, err)
	}
	if !exists(keep) {
		t.Fatal("junction target was deleted")
	}
	if exists(filepath.Join(item, "link")) || !exists(item) {
		t.Fatal("link not removed or root removed")
	}
}

func TestRemoveAllRootIsJunction(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "victim", "keep.txt")
	writeFile(t, keep, 5)
	link := filepath.Join(root, "link")
	mkJunction(t, link, filepath.Join(root, "victim"))
	freed, _, err := OSDeleter().RemoveAll(link)
	if err != nil || freed != 0 {
		t.Fatalf("freed %d err %v", freed, err)
	}
	if exists(link) {
		t.Fatal("link not removed")
	}
	if !exists(keep) {
		t.Fatal("junction target was deleted")
	}
}

func TestRemoveContentsRootIsJunction(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "victim", "keep.txt")
	writeFile(t, keep, 5)
	link := filepath.Join(root, "link")
	mkJunction(t, link, filepath.Join(root, "victim"))
	_, _, err := OSDeleter().RemoveContents(link)
	if !errors.Is(err, ErrReparseRoot) {
		t.Fatalf("err = %v", err)
	}
	if !exists(keep) || !exists(link) {
		t.Fatal("something was deleted")
	}
}

func TestRecycleMovesToBin(t *testing.T) {
	if os.Getenv("CI_NO_RECYCLE") == "1" {
		t.Skip("CI_NO_RECYCLE=1")
	}
	f := filepath.Join(t.TempDir(), "nodegarbage-recycle-test.txt")
	writeFile(t, f, 16)
	if err := OSDeleter().Recycle(f); err != nil {
		t.Fatal(err)
	}
	if exists(f) {
		t.Fatal("file still at its original path")
	}
}

func TestRecyclePathTooLong(t *testing.T) {
	// Nothing is created: the length check happens before any shell call.
	p := filepath.Join(t.TempDir(), strings.Repeat("a", 250), "b.txt")
	if err := OSDeleter().Recycle(p); !errors.Is(err, ErrPathTooLong) {
		t.Fatalf("err = %v", err)
	}
}

// Review focus 2: the item turned into a junction between planning and
// execution. Recheck refuses it and the target survives.
func TestExecuteItemBecameJunction(t *testing.T) {
	root := t.TempDir()
	itemPath := filepath.Join(root, "item")
	writeFile(t, filepath.Join(itemPath, "a.txt"), 3)
	keep := filepath.Join(root, "victim", "keep.txt")
	writeFile(t, keep, 5)

	pl := NewPlanner(rules.NewProtector(os.Getenv, nil, nil), OSDeps(), func(string) bool { return true }, nil)
	p, err := pl.Plan([]Request{{Path: itemPath, Size: 3, Class: clsSafeDir}}, ModePermanent, nil)
	if err != nil || len(p.Items) != 1 {
		t.Fatalf("plan %+v err %v", p, err)
	}
	p, err = pl.Redeem(p.ID, p.Token)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(itemPath); err != nil {
		t.Fatal(err)
	}
	mkJunction(t, p.Items[0].Path, filepath.Join(root, "victim"))

	res := Execute(context.Background(), p, OSDeleter(), pl.Recheck, nil)
	if res[0].Status != StatusSkipped || res[0].Reason != ReasonChanged {
		t.Fatalf("result = %+v", res[0])
	}
	if !exists(keep) {
		t.Fatal("junction target was deleted")
	}

	// Gone entirely: not_found.
	if err := os.Remove(p.Items[0].Path); err != nil {
		t.Fatal(err)
	}
	res = Execute(context.Background(), p, OSDeleter(), pl.Recheck, nil)
	if res[0].Status != StatusSkipped || res[0].Reason != ReasonNotFound {
		t.Fatalf("result = %+v", res[0])
	}
}

func TestExecuteRealPermanent(t *testing.T) {
	root := t.TempDir()
	itemPath := filepath.Join(root, "node_modules")
	writeFile(t, filepath.Join(itemPath, "x", "y.js"), 9)
	pl := NewPlanner(rules.NewProtector(os.Getenv, nil, nil), OSDeps(), func(string) bool { return true }, nil)
	p, err := pl.Plan([]Request{{Path: itemPath, Size: 9, Class: clsSafeDir}}, ModePermanent, nil)
	if err != nil || len(p.Items) != 1 {
		t.Fatalf("plan %+v err %v", p, err)
	}
	res := Execute(context.Background(), p, OSDeleter(), pl.Recheck, nil)
	if res[0].Status != StatusOK || res[0].Freed != 9 || exists(itemPath) {
		t.Fatalf("result = %+v", res[0])
	}
}

// A directory swapped for a junction to the victim after it was listed and
// while its children are processed: the walk keeps working on the handle it
// opened, so the victim's same-named file survives.
func TestRemoveAllSwapDuringWalk(t *testing.T) {
	root := t.TempDir()
	victim := filepath.Join(root, "victim")
	keep := filepath.Join(victim, "keep.txt")
	writeFile(t, keep, 5)
	item := filepath.Join(root, "item")
	sub := filepath.Join(item, "sub")
	writeFile(t, filepath.Join(sub, "keep.txt"), 7)
	writeFile(t, filepath.Join(sub, "deeper", "keep.txt"), 1)

	swapped := false
	afterListHook = func(dir string) {
		if swapped || !strings.EqualFold(filepath.Base(dir), "sub") {
			return
		}
		swapped = true
		if err := os.Rename(sub, filepath.Join(item, "moved")); err != nil {
			t.Errorf("rename during walk: %v", err)
			return
		}
		mkJunction(t, sub, victim)
	}
	t.Cleanup(func() { afterListHook = nil })

	freed, _, err := OSDeleter().RemoveAll(item)
	if !swapped {
		t.Fatal("hook did not run")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !exists(keep) {
		t.Fatal("victim file deleted through a junction swapped in during the walk")
	}
	if freed != 8 {
		t.Fatalf("freed = %d, want 8 (the moved directory's files)", freed)
	}
	if exists(filepath.Join(item, "moved")) {
		t.Fatal("moved directory (the one actually walked) not removed")
	}
}

// Same swap with RemoveContents.
func TestRemoveContentsSwapDuringWalk(t *testing.T) {
	root := t.TempDir()
	victim := filepath.Join(root, "victim")
	keep := filepath.Join(victim, "keep.txt")
	writeFile(t, keep, 5)
	item := filepath.Join(root, "item")
	sub := filepath.Join(item, "sub")
	writeFile(t, filepath.Join(sub, "keep.txt"), 7)

	afterListHook = func(dir string) {
		if !strings.EqualFold(filepath.Base(dir), "sub") || exists(filepath.Join(item, "moved")) {
			return
		}
		if err := os.Rename(sub, filepath.Join(item, "moved")); err != nil {
			t.Errorf("rename during walk: %v", err)
			return
		}
		mkJunction(t, sub, victim)
	}
	t.Cleanup(func() { afterListHook = nil })
	if _, _, err := OSDeleter().RemoveContents(item); err != nil {
		t.Fatal(err)
	}
	if !exists(keep) {
		t.Fatal("victim file deleted")
	}
}

func isReadOnly(t *testing.T, path string) bool {
	t.Helper()
	p, _ := windows.UTF16PtrFromString(path)
	a, err := windows.GetFileAttributes(p)
	if err != nil {
		t.Fatal(err)
	}
	return a&windows.FILE_ATTRIBUTE_READONLY != 0
}

// Read-only handling never changes a link target's attributes.
func TestReadOnlyNeverTouchesLinkTarget(t *testing.T) {
	root := t.TempDir()
	victim := filepath.Join(root, "victim")
	ro := filepath.Join(victim, "ro.txt")
	writeFile(t, ro, 4)
	if err := os.Chmod(ro, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o644) })
	item := filepath.Join(root, "item")
	writeFile(t, filepath.Join(item, "own.txt"), 2)
	mkJunction(t, filepath.Join(item, "dirlink"), victim)
	fileLink := filepath.Join(item, "filelink.txt")
	haveFileLink := os.Symlink(ro, fileLink) == nil // needs Developer Mode or admin
	if !haveFileLink {
		t.Log("file symlink not created (no privilege); junction case only")
	}

	freed, skipped, err := OSDeleter().RemoveAll(item)
	if err != nil || len(skipped) != 0 || freed != 2 {
		t.Fatalf("freed %d skipped %v err %v", freed, skipped, err)
	}
	if !exists(ro) || !isReadOnly(t, ro) {
		t.Fatal("link target deleted or its read-only attribute changed")
	}
	if exists(item) {
		t.Fatal("item still exists")
	}
}

func TestRemoveAllReadOnlyDir(t *testing.T) {
	x := filepath.Join(t.TempDir(), "x")
	writeFile(t, filepath.Join(x, "d", "f.txt"), 3)
	d := filepath.Join(x, "d")
	p, _ := windows.UTF16PtrFromString(d)
	if err := windows.SetFileAttributes(p, windows.FILE_ATTRIBUTE_READONLY); err != nil {
		t.Fatal(err)
	}
	freed, skipped, err := OSDeleter().RemoveAll(x)
	if err != nil || len(skipped) != 0 || freed != 3 || exists(x) {
		t.Fatalf("freed %d skipped %v err %v exists %v", freed, skipped, err, exists(x))
	}
}

func TestDirInfoLayout(t *testing.T) {
	var e fileIDBothDirInfo
	if unsafe.Offsetof(e.FileID) != 96 || unsafe.Offsetof(e.FileName) != 104 {
		t.Fatalf("FILE_ID_BOTH_DIR_INFO layout: FileId %d FileName %d", unsafe.Offsetof(e.FileID), unsafe.Offsetof(e.FileName))
	}
}

// The FileDispositionInfo fallback (no IGNORE_READONLY support) clears
// read-only through the entry's own handle, never through a link.
func TestRemoveAllDispositionFallback(t *testing.T) {
	forceDispositionFallback = true
	t.Cleanup(func() { forceDispositionFallback = false })
	t.Run("readonly", TestRemoveAllReadOnly)
	t.Run("readonly dir", TestRemoveAllReadOnlyDir)
	t.Run("link target", TestReadOnlyNeverTouchesLinkTarget)
	t.Run("junction", TestRemoveAllDoesNotFollowJunction)
	t.Run("locked", TestRemoveAllSkipsLocked)
	t.Run("swap", TestRemoveAllSwapDuringWalk)
}
