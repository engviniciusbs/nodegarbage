//go:build windows

package scanner

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

func TestOSFSJunctionAndLongPath(t *testing.T) {
	base := t.TempDir()
	deep := base + `\deep`
	for len(deep) < 300 {
		deep += `\` + strings.Repeat("d", 20)
	}
	if err := os.MkdirAll(`\\?\`+deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(`\\?\`+deep+`\f.bin`, make([]byte, 10), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", base+`\j`, base+`\deep`).CombinedOutput(); err != nil {
		t.Skipf("mklink /J failed: %v: %s", err, out)
	}

	b := tree.NewBuilder(base, 10)
	if err := (&WalkScanner{FS: OSFS()}).Scan(context.Background(), base, b, &Progress{}); err != nil {
		t.Fatal(err)
	}
	tr := b.Finish()
	root := tr.Nodes[tr.Root]
	if root.TotalSize != 10 || root.TotalFiles != 1 {
		t.Fatalf("size=%d files=%d", root.TotalSize, root.TotalFiles)
	}
	id, ok := tr.Find(base + `\j`)
	if !ok || tr.Nodes[id].Flags&tree.FlagLink == 0 {
		t.Fatalf("junction not flagged: ok=%v", ok)
	}
	if len(tr.TopFiles) != 1 || strings.HasPrefix(tr.TopFiles[0].Path, `\\?\`) {
		t.Fatalf("top files: %+v", tr.TopFiles)
	}
}
