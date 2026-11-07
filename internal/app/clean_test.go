package app

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/cleaner"
	"github.com/engviniciusbs/nodegarbage/internal/rules"
)

// fakeCleanDeps canonicalizes every path to itself unless vanish is set.
type fakeCleanDeps struct{ vanish bool }

func (f *fakeCleanDeps) Canonicalize(p string) (string, error) {
	if f.vanish {
		return "", errors.New("not found")
	}
	return p, nil
}
func (*fakeCleanDeps) ContainsGit(string) bool { return false }
func (*fakeCleanDeps) RecycleBinRemaining(string) (int64, bool, error) {
	return 1 << 40, true, nil
}
func (*fakeCleanDeps) VolumeOf(string) string { return `C:\` }

// fakeDeleter counts calls; block, when set, holds every Recycle/RemoveAll.
type fakeDeleter struct {
	mu    sync.Mutex
	calls []string
	block chan struct{}
	enter chan struct{}
}

func (d *fakeDeleter) record(op, p string) {
	d.mu.Lock()
	d.calls = append(d.calls, op+" "+p)
	d.mu.Unlock()
	if d.enter != nil {
		d.enter <- struct{}{}
	}
	if d.block != nil {
		<-d.block
	}
}
func (d *fakeDeleter) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.calls)
}
func (d *fakeDeleter) Recycle(p string) error { d.record("recycle", p); return nil }
func (d *fakeDeleter) RemoveAll(p string) (int64, []string, error) {
	d.record("removeall", p)
	return 1, nil, nil
}
func (d *fakeDeleter) RemoveContents(p string) (int64, []string, error) {
	d.record("removecontents", p)
	return 1, nil, nil
}

// newCleanApp returns a scanned app wired with a real planner over fake FS deps.
func newCleanApp(t *testing.T) (*App, *fakeDeleter, *rec) {
	a, d, r, _ := newCleanAppDeps(t)
	return a, d, r
}

func newCleanAppDeps(t *testing.T) (*App, *fakeDeleter, *rec, *fakeCleanDeps) {
	t.Helper()
	a, r := newTestApp(t, fakeScanner{}, nil)
	if err := a.StartScan([]string{`C:\`}); err != nil {
		t.Fatal(err)
	}
	a.waitScan()
	d := &fakeDeleter{}
	a.deps.Deleter = d
	fd := &fakeCleanDeps{}
	a.deps.Planner = cleaner.NewPlanner(a.deps.Protector, fd, a.IsScanned, nil)
	return a, d, r, fd
}

func mustPlan(t *testing.T, a *App, mode string, paths ...string) PlanDTO {
	t.Helper()
	p, err := a.PlanClean(paths, mode)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPlanCleanTakesSizeAndClassFromSession(t *testing.T) {
	a, _, _ := newCleanApp(t)
	p := mustPlan(t, a, "recycle", `c:\PROJ\node_modules`, `C:\nowhere`)
	if len(p.Items) != 1 || p.Items[0].Size != 1500 || p.Items[0].RuleID != "node.node_modules" ||
		p.Items[0].Verdict != "safe" || p.Items[0].Source != "rule" || p.Items[0].Action != "delete_dir" {
		t.Fatalf("items=%+v", p.Items)
	}
	if len(p.Refused) != 1 || p.Refused[0].Reason != cleaner.ReasonNotScanned || p.Refused[0].Path != `C:\nowhere` {
		t.Fatalf("refused=%+v", p.Refused)
	}
	if p.Token == "" || p.ID == "" || p.TotalBytes != 1500 {
		t.Fatalf("plan=%+v", p)
	}
	if _, err := a.PlanClean([]string{`C:\other`}, "bogus"); err == nil {
		t.Fatal("expected invalid mode error")
	}
}

func TestExecuteRequiresValidToken(t *testing.T) {
	a, d, _ := newCleanApp(t)
	p := mustPlan(t, a, "recycle", `C:\proj\node_modules`)
	if err := a.ExecuteClean(p.ID, "errado"); err == nil {
		t.Fatal("expected error for wrong token")
	}
	if err := a.ExecuteClean("nope", p.Token); err == nil {
		t.Fatal("expected error for unknown plan")
	}
	// the wrong attempt consumed the plan: even the right token fails now
	if err := a.ExecuteClean(p.ID, p.Token); err == nil {
		t.Fatal("plan must be single-use")
	}
	a.waitClean()
	if d.count() != 0 {
		t.Fatalf("deleter called: %v", d.calls)
	}
}

func TestCleanRecordsHistoryAndUpdatesTree(t *testing.T) {
	a, d, r := newCleanApp(t)
	p := mustPlan(t, a, "recycle", `C:\proj\node_modules`, `C:\other`)
	if len(p.Items) != 2 {
		t.Fatalf("plan=%+v", p)
	}
	if err := a.ExecuteClean(p.ID, p.Token); err != nil {
		t.Fatal(err)
	}
	a.waitClean()
	if d.count() != 2 {
		t.Fatalf("calls=%v", d.calls)
	}
	h, err := a.GetHistory(10)
	if err != nil || len(h.Items) != 2 || h.TotalFreed != 1550 {
		t.Fatalf("history=%+v err=%v", h, err)
	}
	for _, it := range h.Items {
		if it.Status != "ok" || it.Mode != "recycle" || it.At.IsZero() {
			t.Fatalf("item=%+v", it)
		}
	}
	nm := h.Items[0]
	if !strings.EqualFold(nm.Path, `C:\other`) && !strings.EqualFold(nm.Path, `C:\proj\node_modules`) {
		t.Fatalf("path=%q", nm.Path)
	}

	root, _ := a.GetChildren(`C:\`, -1)
	if len(root) != 1 || root[0].Name != "proj" || root[0].Size != 30 {
		t.Fatalf("root=%+v", root)
	}
	kids, _ := a.GetChildren(`C:\`, root[0].ID)
	if len(kids) != 1 || kids[0].Name != "src" {
		t.Fatalf("kids=%+v", kids)
	}
	if s, _ := a.GetSummary(`C:\`); s.SafeBytes != 0 {
		t.Fatalf("summary=%+v", s)
	}
	if len(r.of(EvCleanProgress)) < 1 {
		t.Fatal("no progress event")
	}
	done := r.of(EvCleanDone)
	if len(done) != 1 || done[0].(CleanDoneDTO).Freed != 1550 || len(done[0].(CleanDoneDTO).Results) != 2 {
		t.Fatalf("done=%+v", done)
	}
	// removed node is no longer plannable
	p2 := mustPlan(t, a, "recycle", `C:\other`)
	if len(p2.Items) != 0 || len(p2.Refused) != 1 || p2.Refused[0].Reason != cleaner.ReasonNotScanned {
		t.Fatalf("p2=%+v", p2)
	}
}

func TestCleanSkipsItemThatVanished(t *testing.T) {
	a, d, _, fd := newCleanAppDeps(t)
	p := mustPlan(t, a, "recycle", `C:\other`)
	fd.vanish = true // the item disappears between planning and execution
	if err := a.ExecuteClean(p.ID, p.Token); err != nil {
		t.Fatal(err)
	}
	a.waitClean()
	if d.count() != 0 {
		t.Fatal("deleter called for a vanished item")
	}
	h, _ := a.GetHistory(10)
	if len(h.Items) != 1 || h.Items[0].Status != "skipped" || h.Items[0].Reason != cleaner.ReasonNotFound || h.TotalFreed != 0 {
		t.Fatalf("history=%+v", h)
	}
	if root, _ := a.GetChildren(`C:\`, -1); len(root) != 2 {
		t.Fatalf("tree changed: %+v", root)
	}
}

func TestApplyCleanResultsDeleteContentsKeepsNode(t *testing.T) {
	a, _, _ := newCleanApp(t)
	plan := cleaner.Plan{Items: []cleaner.PlanItem{{Path: `C:\proj\node_modules`, Class: rules.Classification{Action: rules.ActionDeleteContents}}}}
	a.applyCleanResults(plan, []cleaner.ItemResult{{Path: `C:\proj\node_modules`, Status: cleaner.StatusOK}})
	root, _ := a.GetChildren(`C:\`, -1)
	proj := findNode(t, root, "proj")
	if proj.Size != 30 {
		t.Fatalf("root=%+v", root)
	}
	kids, _ := a.GetChildren(`C:\`, proj.ID)
	nm := findNode(t, kids, "node_modules")
	if nm.Size != 0 || nm.HasChildren {
		t.Fatalf("nm=%+v", nm)
	}
}

func TestPartialResultStaysInTree(t *testing.T) {
	a, _, _ := newCleanApp(t)
	plan := cleaner.Plan{Items: []cleaner.PlanItem{{Path: `C:\other`}}}
	a.applyCleanResults(plan, []cleaner.ItemResult{{Path: `C:\other`, Status: cleaner.StatusOK, Reason: cleaner.ReasonPartial, SkippedCount: 1}})
	if root, _ := a.GetChildren(`C:\`, -1); len(root) != 2 {
		t.Fatalf("root=%+v", root)
	}
}

func TestOneCleanAtATimeAndCancel(t *testing.T) {
	a, d, r := newCleanApp(t)
	d.block, d.enter = make(chan struct{}), make(chan struct{}, 4)
	p := mustPlan(t, a, "recycle", `C:\proj\node_modules`, `C:\other`)
	p2 := mustPlan(t, a, "recycle", `C:\proj\src`)
	if err := a.ExecuteClean(p.ID, p.Token); err != nil {
		t.Fatal(err)
	}
	select {
	case <-d.enter:
	case <-time.After(5 * time.Second):
		t.Fatal("deleter not reached")
	}
	if err := a.ExecuteClean(p2.ID, p2.Token); err == nil {
		t.Fatal("expected error while a clean is running")
	}
	a.CancelClean()
	close(d.block)
	a.waitClean()
	if d.count() != 1 {
		t.Fatalf("calls=%v: remaining item must be skipped", d.calls)
	}
	res := r.of(EvCleanDone)[0].(CleanDoneDTO).Results
	if len(res) != 2 || res[1].Status != "skipped" || res[1].Reason != cleaner.ReasonCanceled {
		t.Fatalf("results=%+v", res)
	}
	// the rejected attempt must not have burned its plan
	d.block, d.enter = nil, nil
	if err := a.ExecuteClean(p2.ID, p2.Token); err != nil {
		t.Fatalf("plan consumed by the rejected attempt: %v", err)
	}
	a.waitClean()
}

// TestOnlyExecuteCleanDeletes calls every other exported binding, with
// plausible arguments, and requires that the Deleter is never touched.
func TestOnlyExecuteCleanDeletes(t *testing.T) {
	a, d, _ := newCleanApp(t)
	a.deps.NewProvider = (&fakeProvider{verdict: rules.Safe}).factory
	a.deps.SecretGet = func(string) (string, error) { return "k", nil }
	a.deps.SecretSet = func(string, string) error { return nil }
	a.deps.SecretDelete = func(string) error { return nil }
	if err := a.SaveSettings(SettingsDTO{AIMinBytes: 1}); err != nil {
		t.Fatal(err)
	}
	_, err := a.SaveAIProfile(AIProfileDTO{Name: "p", Kind: "anthropic", Model: "m"}, "k")
	if err != nil {
		t.Fatal(err)
	}

	typ := reflect.TypeOf(a)
	called := 0
	for pass := 0; pass < 4; pass++ {
		for i := 0; i < typ.NumMethod(); i++ {
			m := typ.Method(i)
			if m.Name == "Startup" || m.Name == "ExecuteClean" {
				continue
			}
			cur, _ := a.SaveAIProfile(AIProfileDTO{Name: "p", Kind: "anthropic", Model: "m"}, "k")
			args := []reflect.Value{reflect.ValueOf(a)}
			for j := 1; j < m.Type.NumIn(); j++ {
				args = append(args, plausibleArg(m.Type.In(j), pass, cur.ID))
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("%s panicked: %v", m.Name, r)
					}
				}()
				m.Func.Call(args)
				called++
			}()
			a.waitAI()
			a.waitScan()
		}
	}
	if called < 30 {
		t.Fatalf("only %d calls: reflection did not enumerate the bindings", called)
	}
	if d.count() != 0 {
		t.Fatalf("Deleter called by a non-ExecuteClean binding: %v", d.calls)
	}
}

func plausibleArg(t reflect.Type, pass int, profileID string) reflect.Value {
	switch t.Kind() {
	case reflect.String:
		return reflect.ValueOf([]string{`C:\proj\node_modules`, "recycle", `C:\`, profileID}[pass])
	case reflect.Slice:
		if t.Elem().Kind() == reflect.String {
			return reflect.ValueOf([]string{`C:\proj\node_modules`, `C:\other`})
		}
	case reflect.Int32:
		return reflect.ValueOf(int32(-1))
	case reflect.Int:
		return reflect.ValueOf(10)
	case reflect.Bool:
		return reflect.ValueOf(true)
	case reflect.Struct:
		v := reflect.New(t).Elem()
		if t == reflect.TypeOf(AIProfileDTO{}) {
			v.Set(reflect.ValueOf(AIProfileDTO{ID: profileID, Name: "p", Kind: "anthropic", Model: "m"}))
		}
		return v
	}
	return reflect.Zero(t)
}
