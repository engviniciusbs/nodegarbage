//go:build windows

package cleaner

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
)

func mkJunction(t *testing.T, link, target string) {
	t.Helper()
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Skipf("mklink /J failed: %v %s", err, out)
	}
	// Remove the link itself (never its target) before TempDir cleanup.
	t.Cleanup(func() { _ = os.Remove(link) })
}

func TestOSDepsCanonicalize(t *testing.T) {
	d := OSDeps()
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	mkJunction(t, link, target)

	ct, err := d.Canonicalize(target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(ct, `\\?\`) || !filepath.IsAbs(ct) {
		t.Fatalf("canonical = %q", ct)
	}
	cl, err := d.Canonicalize(link)
	if err != nil {
		t.Fatal(err)
	}
	if cl != ct {
		t.Fatalf("junction resolved to %q, want %q", cl, ct)
	}
	// Case and separators don't matter.
	cu, err := d.Canonicalize(strings.ReplaceAll(strings.ToUpper(target), `\`, `/`) + `/`)
	if err != nil || cu != ct {
		t.Fatalf("upper = %q, %v", cu, err)
	}
	if _, err := d.Canonicalize(filepath.Join(root, "missing")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing err = %v", err)
	}
	if _, err := d.Canonicalize(filepath.Join(root, "missing", "deeper")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing parent err = %v", err)
	}
	if got := d.VolumeOf(ct); got != strings.ToUpper(ct[:1])+`:\` {
		t.Fatalf("VolumeOf = %q", got)
	}
}

func TestOSDepsContainsGit(t *testing.T) {
	d := OSDeps()
	root := t.TempDir()
	plain := filepath.Join(root, "plain", "a", "b")
	repo := filepath.Join(root, "repo", "sub", ".GIT")
	other := filepath.Join(root, "other")
	for _, p := range []string{plain, repo, other} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(plain, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if d.ContainsGit(filepath.Join(root, "plain")) {
		t.Fatal("plain tree reported .git")
	}
	if !d.ContainsGit(filepath.Join(root, "repo")) {
		t.Fatal(".git not found")
	}
	// A junction to the repo is not followed.
	mkJunction(t, filepath.Join(other, "j"), filepath.Join(root, "repo"))
	if d.ContainsGit(other) {
		t.Fatal("junction was followed")
	}
}

func TestOSDepsRecycleBinSystemDrive(t *testing.T) {
	d := OSDeps()
	sys := os.Getenv("SystemDrive") + `\`
	rem, has, err := d.RecycleBinRemaining(sys)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s remaining=%d hasBin=%v", sys, rem, has)
	if has && rem < 0 {
		t.Fatalf("remaining = %d", rem)
	}
}

func TestOSDepsJunctionIntoProtected(t *testing.T) {
	// End-to-end with the real Deps: a junction pointing into a protected
	// subtree is refused as protected. The protected target is a temp dir
	// (user extra), so no junction to real system folders is ever created.
	root := t.TempDir()
	target := filepath.Join(root, "protected")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "win")
	mkJunction(t, link, target)
	canonTarget, err := OSDeps().Canonicalize(target) // %TEMP% may be an 8.3 path
	if err != nil {
		t.Fatal(err)
	}
	prot := rules.NewProtector(os.Getenv, nil, []string{canonTarget})
	pl := NewPlanner(prot, OSDeps(), func(string) bool { return true }, nil)
	p, err := pl.Plan([]Request{{Path: link, Size: 1, Class: clsUnknown}}, ModePermanent, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := refusalReason(p, link); got != ReasonProtected {
		t.Fatalf("reason = %q (items %+v)", got, p.Items)
	}
}
