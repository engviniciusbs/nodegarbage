package scanner

import (
	"context"
	"errors"
	"io/fs"
	"runtime"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

// fakeFS is an in-memory FS. Safe for concurrent reads.
type fakeFS struct {
	dirs   map[string][]DirEntry
	errs   map[string]error
	forbid map[string]bool // ReadDir on these paths fails the test
	t      *testing.T

	mu     sync.Mutex
	onRead func(path string)
}

func newFake(t *testing.T) *fakeFS {
	return &fakeFS{t: t, dirs: map[string][]DirEntry{}, errs: map[string]error{}, forbid: map[string]bool{}}
}

func (f *fakeFS) ReadDir(path string) ([]DirEntry, error) {
	if f.forbid[path] {
		f.t.Errorf("ReadDir called for forbidden path %q", path)
	}
	f.mu.Lock()
	hook := f.onRead
	f.mu.Unlock()
	if hook != nil {
		hook(path)
	}
	if err, ok := f.errs[path]; ok {
		return nil, err
	}
	return f.dirs[path], nil
}

func dir(name string) DirEntry { return DirEntry{Name: name, IsDir: true} }
func file(name string, size, mod int64) DirEntry {
	return DirEntry{Name: name, Size: size, ModTime: mod}
}

func scan(t *testing.T, f FS, workers int) (*tree.Tree, *Progress, error) {
	t.Helper()
	b := tree.NewBuilder(`C:\`, 10)
	p := &Progress{}
	err := (&WalkScanner{FS: f, Workers: workers}).Scan(context.Background(), `C:\`, b, p)
	return b.Finish(), p, err
}

func basicFake(t *testing.T) *fakeFS {
	f := newFake(t)
	f.dirs[`C:\`] = []DirEntry{dir("a"), dir("b"), file("r.txt", 1, 10)}
	f.dirs[`C:\a`] = []DirEntry{dir("c"), file("a1.bin", 100, 20), file("a2.bin", 200, 30)}
	f.dirs[`C:\a\c`] = []DirEntry{file("c1.log", 1000, 40)}
	f.dirs[`C:\b`] = []DirEntry{file("b1.dat", 10, 5), file("b2.dat", 20, 6)}
	return f
}

func TestWalkSumsTree(t *testing.T) {
	for _, w := range []int{1, 8} {
		tr, _, err := scan(t, basicFake(t), w)
		if err != nil {
			t.Fatal(err)
		}
		root := tr.Nodes[tr.Root]
		if root.TotalSize != 1331 || root.TotalFiles != 6 {
			t.Fatalf("workers=%d: size=%d files=%d", w, root.TotalSize, root.TotalFiles)
		}
		if root.TreeLatest != 40 {
			t.Fatalf("latest=%d", root.TreeLatest)
		}
		id, ok := tr.Find(`C:\a\c`)
		if !ok || tr.Nodes[id].TotalSize != 1000 {
			t.Fatalf("C:\\a\\c missing or wrong: %v", ok)
		}
	}
}

func TestReparsePointNotFollowed(t *testing.T) {
	f := basicFake(t)
	f.dirs[`C:\`] = append(f.dirs[`C:\`], DirEntry{Name: "j", IsDir: true, Attrs: windows.FILE_ATTRIBUTE_REPARSE_POINT})
	f.forbid[`C:\j`] = true
	f.dirs[`C:\j`] = []DirEntry{file("x", 999, 1)}
	tr, _, err := scan(t, f, 4)
	if err != nil {
		t.Fatal(err)
	}
	id, ok := tr.Find(`C:\j`)
	if !ok || tr.Nodes[id].Flags&tree.FlagLink == 0 || tr.Nodes[id].TotalSize != 0 {
		t.Fatalf("junction node wrong: ok=%v %+v", ok, tr.Nodes[id])
	}
	if tr.Nodes[tr.Root].TotalSize != 1331 {
		t.Fatalf("size=%d", tr.Nodes[tr.Root].TotalSize)
	}
}

func TestCloudFilesCountZero(t *testing.T) {
	f := newFake(t)
	f.dirs[`C:\`] = []DirEntry{
		{Name: "big.bin", Size: 1 << 30, ModTime: 1, Attrs: windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS},
		file("small", 5, 2),
		dir("od"),
		{Name: "cloud", IsDir: true, Attrs: windows.FILE_ATTRIBUTE_RECALL_ON_OPEN},
	}
	f.dirs[`C:\od`] = []DirEntry{{Name: "o.bin", Size: 1 << 20, Attrs: windows.FILE_ATTRIBUTE_OFFLINE}, file("l.txt", 7, 3)}
	f.dirs[`C:\cloud`] = []DirEntry{file("k", 3, 1)}
	tr, _, err := scan(t, f, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := tr.Nodes[tr.Root].TotalSize; got != 5+7+3 {
		t.Fatalf("TotalSize=%d", got)
	}
	if got := tr.Nodes[tr.Root].TotalFiles; got != 5 {
		t.Fatalf("TotalFiles=%d", got)
	}
	id, _ := tr.Find(`C:\cloud`)
	if tr.Nodes[id].Flags&tree.FlagCloud == 0 {
		t.Fatal("cloud dir not flagged")
	}
}

func TestAccessDeniedContinues(t *testing.T) {
	f := basicFake(t)
	f.dirs[`C:\`] = append(f.dirs[`C:\`], dir("x"))
	f.errs[`C:\x`] = windows.ERROR_ACCESS_DENIED
	tr, p, err := scan(t, f, 4)
	if err != nil {
		t.Fatal(err)
	}
	id, ok := tr.Find(`C:\x`)
	if !ok || tr.Nodes[id].Flags&tree.FlagDenied == 0 {
		t.Fatal("denied node missing/unflagged")
	}
	if tr.DeniedCount != 1 || len(tr.Denied) != 1 || tr.Denied[0] != `C:\x` {
		t.Fatalf("denied=%v count=%d", tr.Denied, tr.DeniedCount)
	}
	if tr.Nodes[tr.Root].TotalSize != 1331 {
		t.Fatal("siblings not scanned")
	}
	if p.Snapshot().Denied != 1 {
		t.Fatal("progress denied")
	}
}

func TestOtherDirErrorTreatedAsDenied(t *testing.T) {
	f := basicFake(t)
	f.errs[`C:\b`] = errors.New("boom")
	tr, _, err := scan(t, f, 2)
	if err != nil || tr.DeniedCount != 1 {
		t.Fatalf("err=%v denied=%d", err, tr.DeniedCount)
	}
}

func TestVanishedDirIgnored(t *testing.T) {
	for _, e := range []error{windows.ERROR_PATH_NOT_FOUND, windows.ERROR_FILE_NOT_FOUND, fs.ErrNotExist} {
		f := basicFake(t)
		f.errs[`C:\b`] = e
		tr, _, err := scan(t, f, 4)
		if err != nil || tr.DeniedCount != 0 {
			t.Fatalf("%v: err=%v denied=%d", e, err, tr.DeniedCount)
		}
		if tr.Nodes[tr.Root].TotalSize != 1301 {
			t.Fatalf("size=%d", tr.Nodes[tr.Root].TotalSize)
		}
	}
}

func TestRootUnavailable(t *testing.T) {
	for _, e := range []error{windows.ERROR_PATH_NOT_FOUND, windows.ERROR_ACCESS_DENIED, errors.New("io")} {
		f := newFake(t)
		f.errs[`C:\`] = e
		_, _, err := scan(t, f, 2)
		if !errors.Is(err, ErrRootUnavailable) {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestCancel(t *testing.T) {
	before := runtime.NumGoroutine()
	f := newFake(t)
	f.dirs[`C:\`] = []DirEntry{dir("a"), dir("b"), dir("c")}
	for _, d := range []string{`C:\a`, `C:\b`, `C:\c`} {
		f.dirs[d] = []DirEntry{file("f", 1, 1), dir("sub")}
		f.dirs[d+`\sub`] = []DirEntry{file("g", 1, 1)}
	}
	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	f.onRead = func(string) { once.Do(cancel) }
	b := tree.NewBuilder(`C:\`, 10)
	err := (&WalkScanner{FS: f, Workers: 4}).Scan(ctx, `C:\`, b, &Progress{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+2 {
		t.Fatalf("goroutines leaked: before=%d after=%d", before, after)
	}
}

func TestProgressCounts(t *testing.T) {
	_, p, err := scan(t, basicFake(t), 3)
	if err != nil {
		t.Fatal(err)
	}
	s := p.Snapshot()
	if s.Files != 6 || s.Dirs != 4 || s.Bytes != 1331 || s.Denied != 0 {
		t.Fatalf("%+v", s)
	}
	if s.Current == "" {
		t.Fatal("Current empty")
	}
}

func TestEngineConst(t *testing.T) {
	var _ Scanner = (*WalkScanner)(nil)
	if EngineWalk != "walk" {
		t.Fatal()
	}
}
