package app

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
	"github.com/engviniciusbs/nodegarbage/internal/scanner"
	"github.com/engviniciusbs/nodegarbage/internal/store"
	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

type rec struct {
	mu sync.Mutex
	ev []recEvent
}

type recEvent struct {
	name string
	data any
}

func (r *rec) Emit(event string, data any) {
	r.mu.Lock()
	r.ev = append(r.ev, recEvent{event, data})
	r.mu.Unlock()
}

func (r *rec) of(name string) []any {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []any
	for _, e := range r.ev {
		if e.name == name {
			out = append(out, e.data)
		}
	}
	return out
}

// fakeScanner feeds a small tree; err, when set, is returned instead.
type fakeScanner struct {
	err   error
	block chan struct{}
}

func (f fakeScanner) Scan(ctx context.Context, root string, b *tree.Builder, p *scanner.Progress) error {
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if f.err != nil {
		return f.err
	}
	r := b.AddDir(tree.NoNode, root, 0)
	proj := b.AddDir(r, "proj", 0)
	b.SetOwnStats(proj, 10, 1, 100)
	nm := b.AddDir(proj, "node_modules", 0)
	b.SetOwnStats(nm, 1000, 5, 100)
	inner := b.AddDir(nm, "node_modules", 0)
	b.SetOwnStats(inner, 500, 2, 100)
	src := b.AddDir(proj, "src", 0)
	b.SetOwnStats(src, 20, 1, 100)
	other := b.AddDir(r, "other", 0)
	b.SetOwnStats(other, 50, 1, 100)
	return nil
}

func testEnv(string) string { return "" }

func newTestApp(t *testing.T, sc scanner.Scanner, st *store.Store) (*App, *rec) {
	t.Helper()
	if st == nil {
		var err error
		st, err = store.Open(filepath.Join(t.TempDir(), "t.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { st.Close() })
	}
	prot := rules.NewProtector(testEnv, []string{`C:\`}, nil)
	rs := []rules.Rule{{
		ID: "node.node_modules", Name: "node_modules", Category: "build-artifact",
		Match:   rules.Match{Dirname: "node_modules", Sibling: "package.json"},
		Verdict: rules.Safe, Action: rules.ActionDeleteDir,
		Regenerable: "npm install", Explain: "Dependências do Node.",
	}}
	list := func(dir string) []string {
		if strings.EqualFold(strings.TrimRight(dir, `\`), `C:\proj`) {
			return []string{"package.json", "node_modules", "src"}
		}
		return nil
	}
	cl := rules.NewClassifier(rs, prot, testEnv, func(string) bool { return false }, list)
	r := &rec{}
	a := New(Deps{
		Store: st, Rules: rs, Protector: prot, Classifier: cl, Emitter: r,
		NewScanner: func(string) scanner.Scanner { return sc },
		Now:        time.Now,
	})
	return a, r
}

func TestStartScanEmitsAndSaves(t *testing.T) {
	a, r := newTestApp(t, fakeScanner{}, nil)
	if err := a.StartScan([]string{`C:\`}); err != nil {
		t.Fatal(err)
	}
	a.waitScan()
	if len(r.of(EvScanProgress)) < 1 {
		t.Fatal("no progress event")
	}
	done := r.of(EvScanDone)
	if len(done) != 1 || done[0].(ScanDoneDTO).Error != "" {
		t.Fatalf("done=%v", done)
	}
	sn, err := a.deps.Store.LoadSnapshot(context.Background(), `C:\`)
	if err != nil || sn == nil {
		t.Fatalf("snapshot missing: %v", err)
	}
}

func TestScanRootUnavailableNotSaved(t *testing.T) {
	a, r := newTestApp(t, fakeScanner{err: scanner.ErrRootUnavailable}, nil)
	if err := a.StartScan([]string{`C:\`}); err != nil {
		t.Fatal(err)
	}
	a.waitScan()
	done := r.of(EvScanDone)
	if len(done) != 1 || done[0].(ScanDoneDTO).Error == "" {
		t.Fatalf("done=%v", done)
	}
	if sn, _ := a.deps.Store.LoadSnapshot(context.Background(), `C:\`); sn != nil {
		t.Fatal("snapshot saved on error")
	}
	if a.IsScanned(`C:\`) {
		t.Fatal("session replaced on error")
	}
}

func TestStartScanWhileRunning(t *testing.T) {
	block := make(chan struct{})
	a, _ := newTestApp(t, fakeScanner{block: block}, nil)
	if err := a.StartScan([]string{`C:\`}); err != nil {
		t.Fatal(err)
	}
	if err := a.StartScan([]string{`C:\`}); err == nil {
		t.Fatal("expected error while running")
	}
	close(block)
	a.waitScan()
	if err := a.StartScan(nil); err == nil {
		t.Fatal("expected error for empty roots")
	}
}

func TestCancelScan(t *testing.T) {
	a, r := newTestApp(t, fakeScanner{block: make(chan struct{})}, nil)
	if err := a.StartScan([]string{`C:\`, `D:\`}); err != nil {
		t.Fatal(err)
	}
	a.CancelScan()
	a.waitScan()
	done := r.of(EvScanDone)
	if len(done) != 1 || done[0].(ScanDoneDTO).Error != "canceled" {
		t.Fatalf("done=%v", done)
	}
}
