package tree

import (
	"sync"
	"testing"
)

func TestFinishAggregatesPostOrder(t *testing.T) {
	b := NewBuilder(`C:\`, 10)
	root := b.AddDir(NoNode, `C:\`, 0)
	a := b.AddDir(root, "a", 0)
	ab := b.AddDir(a, "b", 0)
	b.SetOwnStats(a, 100, 2, 10)
	b.SetOwnStats(ab, 50, 1, 30)
	tr := b.Finish()
	na := tr.Nodes[a]
	if na.TotalSize != 150 || na.TotalFiles != 3 || na.TreeLatest != 30 {
		t.Fatalf("a = %+v", na)
	}
	if tr.Nodes[tr.Root].TotalSize != 150 {
		t.Fatalf("root total = %d", tr.Nodes[tr.Root].TotalSize)
	}
}

func TestTopFilesMergedAcrossWorkers(t *testing.T) {
	b := NewBuilder(`C:\`, 3)
	b.AddDir(NoNode, `C:\`, 0)
	w1, w2 := b.NewWorker(), b.NewWorker()
	for _, s := range []int64{5, 1, 9} {
		w1.AddFile(`C:\x`, FileEntry{Name: "f", Size: s})
	}
	for _, s := range []int64{7, 3} {
		w2.AddFile(`C:\y`, FileEntry{Name: "g", Size: s})
	}
	tr := b.Finish()
	want := []int64{9, 7, 5}
	if len(tr.TopFiles) != 3 {
		t.Fatalf("len = %d", len(tr.TopFiles))
	}
	for i, s := range want {
		if tr.TopFiles[i].Size != s {
			t.Fatalf("TopFiles[%d] = %d, want %d", i, tr.TopFiles[i].Size, s)
		}
	}
	if tr.TopFiles[0].Path != `C:\x\f` || tr.TopFiles[1].Path != `C:\y\g` {
		t.Fatalf("paths: %+v", tr.TopFiles)
	}
}

func TestExtensionsCaseInsensitive(t *testing.T) {
	b := NewBuilder(`C:\`, 10)
	b.AddDir(NoNode, `C:\`, 0)
	w := b.NewWorker()
	w.AddFile(`C:\`, FileEntry{Name: "A.LOG", Size: 10})
	w.AddFile(`C:\`, FileEntry{Name: "b.log", Size: 5})
	w.AddFile(`C:\`, FileEntry{Name: "Makefile", Size: 1})
	tr := b.Finish()
	if len(tr.Extensions) != 2 {
		t.Fatalf("%+v", tr.Extensions)
	}
	if tr.Extensions[0] != (ExtStat{".log", 2, 15}) || tr.Extensions[1] != (ExtStat{"", 1, 1}) {
		t.Fatalf("%+v", tr.Extensions)
	}
}

func TestDeniedCappedAt100(t *testing.T) {
	b := NewBuilder(`C:\`, 10)
	b.AddDir(NoNode, `C:\`, 0)
	for i := 0; i < 150; i++ {
		b.AddDenied(`C:\x`)
	}
	tr := b.Finish()
	if len(tr.Denied) != 100 || tr.DeniedCount != 150 {
		t.Fatalf("%d %d", len(tr.Denied), tr.DeniedCount)
	}
}

func TestPathAndFind(t *testing.T) {
	b := NewBuilder(`C:\`, 10)
	root := b.AddDir(NoNode, `C:\`, 0)
	a := b.AddDir(root, "a", 0)
	ab := b.AddDir(a, "b", 0)
	tr := b.Finish()
	if p := tr.Path(ab); p != `C:\a\b` {
		t.Fatalf("path = %q", p)
	}
	if p := tr.Path(root); p != `C:\` {
		t.Fatalf("root path = %q", p)
	}
	for _, q := range []string{`c:\A\B`, `c:\A\B\`} {
		if id, ok := tr.Find(q); !ok || id != ab {
			t.Fatalf("Find(%q) = %d,%v", q, id, ok)
		}
	}
	if id, ok := tr.Find(`c:\`); !ok || id != root {
		t.Fatalf("Find root = %d,%v", id, ok)
	}
	if _, ok := tr.Find(`C:\zzz`); ok {
		t.Fatal("found nonexistent")
	}
}

func TestSortedChildren(t *testing.T) {
	b := NewBuilder(`C:\`, 10)
	root := b.AddDir(NoNode, `C:\`, 0)
	x := b.AddDir(root, "x", 0)
	y := b.AddDir(root, "y", 0)
	b.SetOwnStats(x, 1, 1, 0)
	b.SetOwnStats(y, 9, 1, 0)
	tr := b.Finish()
	got := tr.SortedChildren(root)
	if len(got) != 2 || got[0] != y || got[1] != x {
		t.Fatalf("%v", got)
	}
}

func TestAddDirConcurrent(t *testing.T) {
	b := NewBuilder(`C:\`, 10)
	root := b.AddDir(NoNode, `C:\`, 0)
	var wg sync.WaitGroup
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				b.AddDir(root, "d", 0)
			}
		}()
	}
	wg.Wait()
	tr := b.Finish()
	if len(tr.Nodes) != 5001 {
		t.Fatalf("nodes = %d", len(tr.Nodes))
	}
}
