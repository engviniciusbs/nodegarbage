package cleaner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
)

// fakeDeleter records calls and never touches the disk.
type fakeDeleter struct {
	mu      sync.Mutex
	calls   []string
	errs    map[string]error // "op:path" -> error
	freed   int64
	skipped []string
}

func (f *fakeDeleter) record(op, path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, op+":"+path)
	return f.errs[op+":"+path]
}

func (f *fakeDeleter) Recycle(path string) error { return f.record("recycle", path) }

func (f *fakeDeleter) RemoveAll(path string) (int64, []string, error) {
	err := f.record("removeall", path)
	return f.freed, f.skipped, err
}

func (f *fakeDeleter) RemoveContents(path string) (int64, []string, error) {
	err := f.record("contents", path)
	return f.freed, f.skipped, err
}

// okRecheck accepts every item.
func okRecheck(PlanItem) string { return "" }

func TestExecuteNoRecheck(t *testing.T) {
	f := &fakeDeleter{}
	items := []PlanItem{item(`D:\a`, 1, rules.ActionDeleteDir), item(`D:\b`, 1, rules.ActionDeleteContents)}
	calls := 0
	res := Execute(context.Background(), Plan{Mode: ModePermanent, Items: items}, f, nil, func(int, ItemResult) { calls++ })
	if len(f.calls) != 0 || calls != 2 {
		t.Fatalf("deleter calls %v, onItem calls %d", f.calls, calls)
	}
	for _, r := range res {
		if r.Status != StatusSkipped || r.Reason != ReasonNoRecheck {
			t.Fatalf("result = %+v", r)
		}
	}
}

func item(path string, size int64, action rules.Action) PlanItem {
	return PlanItem{Path: path, Size: size, Class: rules.Classification{Verdict: rules.Safe, Action: action}}
}

func TestExecuteModes(t *testing.T) {
	ctx := context.Background()

	f := &fakeDeleter{}
	res := Execute(ctx, Plan{Mode: ModeRecycle, Items: []PlanItem{item(`D:\a\node_modules`, 10, rules.ActionDeleteDir)}}, f, okRecheck, nil)
	if len(f.calls) != 1 || f.calls[0] != `recycle:D:\a\node_modules` {
		t.Fatalf("recycle+delete_dir calls = %v", f.calls)
	}
	if res[0].Status != StatusOK || res[0].Freed != 10 {
		t.Fatalf("recycle result = %+v", res[0])
	}

	f = &fakeDeleter{freed: 7}
	res = Execute(ctx, Plan{Mode: ModePermanent, Items: []PlanItem{item(`D:\cache`, 10, rules.ActionDeleteContents)}}, f, okRecheck, nil)
	if len(f.calls) != 1 || f.calls[0] != `contents:D:\cache` {
		t.Fatalf("permanent+delete_contents calls = %v", f.calls)
	}
	if res[0].Status != StatusOK || res[0].Freed != 7 {
		t.Fatalf("permanent result = %+v", res[0])
	}

	f = &fakeDeleter{freed: 3}
	res = Execute(ctx, Plan{Mode: ModePermanent, Items: []PlanItem{item(`D:\nm`, 10, rules.ActionDeleteDir)}}, f, okRecheck, nil)
	if len(f.calls) != 1 || f.calls[0] != `removeall:D:\nm` || res[0].Freed != 3 {
		t.Fatalf("permanent+delete_dir calls = %v res %+v", f.calls, res[0])
	}

	// recycle+delete_contents: one Recycle per direct child (listing is real,
	// inside TempDir; the fake deletes nothing).
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	f = &fakeDeleter{}
	res = Execute(ctx, Plan{Mode: ModeRecycle, Items: []PlanItem{item(dir, 10, rules.ActionDeleteContents)}}, f, okRecheck, nil)
	got := append([]string(nil), f.calls...)
	sort.Strings(got)
	want := []string{"recycle:" + filepath.Join(dir, "a.txt"), "recycle:" + filepath.Join(dir, "b")}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("recycle per child = %v, want %v", got, want)
	}
	if res[0].Status != StatusOK || res[0].Freed != 10 {
		t.Fatalf("recycle contents result = %+v", res[0])
	}

	// Empty directory: nothing to recycle is fine.
	f = &fakeDeleter{}
	res = Execute(ctx, Plan{Mode: ModeRecycle, Items: []PlanItem{item(t.TempDir(), 0, rules.ActionDeleteContents)}}, f, okRecheck, nil)
	if res[0].Status != StatusOK || len(f.calls) != 0 {
		t.Fatalf("empty contents = %+v calls %v", res[0], f.calls)
	}
}

func TestExecuteRecycleContentsPartial(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f := &fakeDeleter{errs: map[string]error{"recycle:" + b: errors.New("in use")}}
	res := Execute(context.Background(), Plan{Mode: ModeRecycle, Items: []PlanItem{item(dir, 5, rules.ActionDeleteContents)}}, f, okRecheck, nil)
	if r := res[0]; r.Status != StatusOK || r.Reason != ReasonPartial || len(r.SkippedFiles) != 1 || r.SkippedFiles[0] != b {
		t.Fatalf("partial = %+v", r)
	}
	// Every child failing is an error.
	f = &fakeDeleter{errs: map[string]error{"recycle:" + a: errors.New("x"), "recycle:" + b: errors.New("y")}}
	res = Execute(context.Background(), Plan{Mode: ModeRecycle, Items: []PlanItem{item(dir, 5, rules.ActionDeleteContents)}}, f, okRecheck, nil)
	if r := res[0]; r.Status != StatusError || r.Freed != 0 {
		t.Fatalf("all failed = %+v", r)
	}
	// Aborted (user cancelled the permanent-deletion warning): stop there.
	f = &fakeDeleter{errs: map[string]error{"recycle:" + a: ErrRecycleAborted}}
	res = Execute(context.Background(), Plan{Mode: ModeRecycle, Items: []PlanItem{item(dir, 5, rules.ActionDeleteContents)}}, f, okRecheck, nil)
	if r := res[0]; r.Status != StatusSkipped || r.Reason != ReasonRecycleAborted || len(f.calls) != 1 {
		t.Fatalf("aborted = %+v calls %v", r, f.calls)
	}
}

func TestExecuteCancelBetweenItems(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var items []PlanItem
	for i := range 5 {
		items = append(items, item(fmt.Sprintf(`D:\x%d`, i), 1, rules.ActionDeleteDir))
	}
	f := &fakeDeleter{}
	var seen []int
	res := Execute(ctx, Plan{Mode: ModePermanent, Items: items}, f, okRecheck, func(i int, r ItemResult) {
		seen = append(seen, i)
		if i == 0 {
			cancel()
		}
	})
	if len(f.calls) != 1 {
		t.Fatalf("deleter calls after cancel: %v", f.calls)
	}
	if res[0].Status != StatusOK {
		t.Fatalf("item 0 = %+v", res[0])
	}
	for i := 1; i < 5; i++ {
		if res[i].Status != StatusSkipped || res[i].Reason != ReasonCanceled || res[i].Path != items[i].Path {
			t.Fatalf("item %d = %+v", i, res[i])
		}
	}
	if fmt.Sprint(seen) != "[0 1 2 3 4]" {
		t.Fatalf("onItem order = %v", seen)
	}
}

func TestExecuteManyItems(t *testing.T) {
	var items []PlanItem
	for i := range 2000 {
		items = append(items, item(fmt.Sprintf(`D:\x\%d`, i), 1, rules.ActionDeleteDir))
	}
	next := 0
	res := Execute(context.Background(), Plan{Mode: ModeRecycle, Items: items}, &fakeDeleter{}, okRecheck, func(i int, r ItemResult) {
		if i != next {
			t.Fatalf("onItem(%d), want %d", i, next)
		}
		next++
	})
	if len(res) != 2000 || next != 2000 {
		t.Fatalf("results %d, onItem calls %d", len(res), next)
	}
	for i, r := range res {
		if r.Status != StatusOK || r.Path != items[i].Path {
			t.Fatalf("result %d = %+v", i, r)
		}
	}
}

func TestErrorDoesNotAbort(t *testing.T) {
	items := []PlanItem{item(`D:\a`, 1, rules.ActionDeleteDir), item(`D:\b`, 1, rules.ActionDeleteDir), item(`D:\c`, 1, rules.ActionDeleteDir)}
	f := &fakeDeleter{errs: map[string]error{`removeall:D:\b`: errors.New("boom")}}
	res := Execute(context.Background(), Plan{Mode: ModePermanent, Items: items}, f, okRecheck, nil)
	if res[1].Status != StatusError || res[1].Reason != ReasonFailed || res[1].Detail == "" {
		t.Fatalf("item 1 = %+v", res[1])
	}
	if res[0].Status != StatusOK || res[2].Status != StatusOK {
		t.Fatalf("results = %+v", res)
	}
}

func TestExecuteErrorMapping(t *testing.T) {
	cases := []struct {
		mode   Mode
		err    error
		status Status
		reason string
	}{
		{ModeRecycle, ErrRecycleAborted, StatusSkipped, ReasonRecycleAborted},
		{ModeRecycle, ErrPathTooLong, StatusError, ReasonPathTooLongForRecycle},
		{ModeRecycle, fmt.Errorf("x: %w", fs.ErrNotExist), StatusSkipped, ReasonNotFound},
		{ModePermanent, ErrNothingRemoved, StatusSkipped, ReasonLocked},
		{ModePermanent, ErrReparseRoot, StatusSkipped, ReasonChanged},
	}
	for _, c := range cases {
		op := "recycle"
		if c.mode == ModePermanent {
			op = "removeall"
		}
		f := &fakeDeleter{errs: map[string]error{op + `:D:\a`: c.err}}
		r := Execute(context.Background(), Plan{Mode: c.mode, Items: []PlanItem{item(`D:\a`, 9, rules.ActionDeleteDir)}}, f, okRecheck, nil)[0]
		if r.Status != c.status || r.Reason != c.reason || r.Freed != 0 {
			t.Errorf("%v: %+v", c.err, r)
		}
	}
}

func TestExecuteSkippedFiles(t *testing.T) {
	var skipped []string
	for i := range 150 {
		skipped = append(skipped, fmt.Sprintf(`D:\a\f%d`, i))
	}
	f := &fakeDeleter{freed: 4, skipped: skipped}
	r := Execute(context.Background(), Plan{Mode: ModePermanent, Items: []PlanItem{item(`D:\a`, 9, rules.ActionDeleteDir)}}, f, okRecheck, nil)[0]
	if r.Status != StatusOK || r.Reason != ReasonPartial || r.Freed != 4 {
		t.Fatalf("result = %+v", r)
	}
	if len(r.SkippedFiles) != MaxSkippedFiles || r.SkippedCount != 150 {
		t.Fatalf("skipped files %d count %d", len(r.SkippedFiles), r.SkippedCount)
	}
}

func TestExecuteRecheck(t *testing.T) {
	items := []PlanItem{item(`D:\a`, 1, rules.ActionDeleteDir), item(`D:\b`, 1, rules.ActionDeleteDir)}
	f := &fakeDeleter{}
	var checked []string
	res := Execute(context.Background(), Plan{Mode: ModePermanent, Items: items}, f, func(it PlanItem) string {
		checked = append(checked, it.Path)
		if it.Path == `D:\a` {
			return ReasonChanged
		}
		return ""
	}, nil)
	if res[0].Status != StatusSkipped || res[0].Reason != ReasonChanged {
		t.Fatalf("item 0 = %+v", res[0])
	}
	if len(f.calls) != 1 || f.calls[0] != `removeall:D:\b` || res[1].Status != StatusOK {
		t.Fatalf("calls %v res %+v", f.calls, res)
	}
	if len(checked) != 2 {
		t.Fatalf("recheck calls = %v", checked)
	}
}

func TestExecuteInvalidActionOrMode(t *testing.T) {
	f := &fakeDeleter{}
	r := Execute(context.Background(), Plan{Mode: ModePermanent, Items: []PlanItem{item(`D:\a`, 1, rules.ActionManual)}}, f, okRecheck, nil)[0]
	if r.Status != StatusError || r.Reason != ReasonInvalidAction || len(f.calls) != 0 {
		t.Fatalf("result %+v calls %v", r, f.calls)
	}
	r = Execute(context.Background(), Plan{Mode: "x", Items: []PlanItem{item(`D:\a`, 1, rules.ActionDeleteDir)}}, f, okRecheck, nil)[0]
	if r.Status != StatusError || r.Reason != ReasonInvalidMode || len(f.calls) != 0 {
		t.Fatalf("bad mode result %+v calls %v", r, f.calls)
	}
}
