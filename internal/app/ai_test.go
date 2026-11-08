package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/engviniciusbs/nodegarbage/internal/ai"
	"github.com/engviniciusbs/nodegarbage/internal/cleaner"
	"github.com/engviniciusbs/nodegarbage/internal/rules"
	"github.com/engviniciusbs/nodegarbage/internal/scanner"
	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

// fakeProvider records the payloads it receives and marks every unknown item
// with verdict.
type fakeProvider struct {
	mu       sync.Mutex
	payloads []ai.Payload
	verdict  rules.Verdict
	err      error
}

func (f *fakeProvider) factory(ai.Profile, string) (ai.Provider, error) { return f, nil }

func (f *fakeProvider) Analyze(_ context.Context, p ai.Payload) (ai.Analysis, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.payloads = append(f.payloads, p)
	if f.err != nil {
		return ai.Analysis{}, f.err
	}
	an := ai.Analysis{ReportMarkdown: "# relatório"}
	for _, it := range p.Unknown {
		an.Items = append(an.Items, ai.ItemVerdict{Path: it.Path, Verdict: f.verdict, Reason: "teste", Confidence: 0.9})
	}
	return an, nil
}

func (f *fakeProvider) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.payloads)
}

type fakeSecrets struct {
	mu sync.Mutex
	m  map[string]string
}

func (s *fakeSecrets) get(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.m[id]; ok {
		return v, nil
	}
	return "", errors.New("not found")
}
func (s *fakeSecrets) set(id, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[id] = v
	return nil
}
func (s *fakeSecrets) del(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

func newAIApp(t *testing.T) (*App, *fakeProvider, *fakeSecrets, *rec, AIProfileDTO) {
	t.Helper()
	a, r := newTestApp(t, fakeScanner{}, nil)
	if err := a.StartScan([]string{`C:\`}); err != nil {
		t.Fatal(err)
	}
	a.waitScan()
	fp := &fakeProvider{verdict: rules.Safe}
	sec := &fakeSecrets{m: map[string]string{}}
	a.deps.NewProvider = fp.factory
	a.deps.SecretGet, a.deps.SecretSet, a.deps.SecretDelete = sec.get, sec.set, sec.del
	if err := a.SaveSettings(SettingsDTO{AIMinBytes: 1}); err != nil {
		t.Fatal(err)
	}
	p, err := a.SaveAIProfile(AIProfileDTO{Name: "Teste", Kind: "anthropic", Model: "m1"}, "sk-123")
	if err != nil {
		t.Fatal(err)
	}
	return a, fp, sec, r, p
}

func TestProfileKeyNeverInSettings(t *testing.T) {
	a, _, sec, _, p := newAIApp(t)
	if !p.HasKey || p.ID == "" || len(p.ID) != 32 {
		t.Fatalf("profile=%+v", p)
	}
	if sec.m[p.ID] != "sk-123" {
		t.Fatal("SecretSet not called with the profile id")
	}
	raw, ok, err := a.deps.Store.GetSetting(context.Background(), "ai.profiles")
	if err != nil || !ok || strings.Contains(string(raw), "sk-123") {
		t.Fatalf("settings=%s err=%v", raw, err)
	}
	list, _ := a.ListAIProfiles()
	if len(list) != 1 || !list[0].HasKey {
		t.Fatalf("list=%+v", list)
	}
}

func TestSaveProfileEmptyKeyKeepsAndDeleteRemoves(t *testing.T) {
	a, _, sec, _, p := newAIApp(t)
	p.Model = "m2"
	q, err := a.SaveAIProfile(p, "")
	if err != nil || q.ID != p.ID || q.Model != "m2" || !q.HasKey || sec.m[p.ID] != "sk-123" {
		t.Fatalf("q=%+v err=%v", q, err)
	}
	if list, _ := a.ListAIProfiles(); len(list) != 1 {
		t.Fatalf("list=%+v", list)
	}
	if _, err := a.SaveAIProfile(AIProfileDTO{ID: "ghost", Name: "x", Kind: "anthropic", Model: "m"}, ""); err == nil {
		t.Fatal("expected error for unknown id")
	}
	if _, err := a.SaveAIProfile(AIProfileDTO{Name: "x", Kind: "nope", Model: "m"}, ""); err == nil {
		t.Fatal("expected error for invalid kind")
	}
	if err := a.DeleteAIProfile(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := sec.m[p.ID]; ok {
		t.Fatal("secret not deleted")
	}
	if list, _ := a.ListAIProfiles(); len(list) != 0 {
		t.Fatalf("list=%+v", list)
	}
}

func TestPreviewMatchesSent(t *testing.T) {
	a, fp, _, r, p := newAIApp(t)
	pv, err := a.PreviewAIPayload(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pv.Cached || pv.Tokens <= 0 || pv.Model != "m1" || pv.Provider != "anthropic" {
		t.Fatalf("preview=%+v", pv)
	}
	if err := a.RunAI(p.ID, false); err != nil {
		t.Fatal(err)
	}
	a.waitAI()
	if fp.calls() != 1 {
		t.Fatalf("calls=%d", fp.calls())
	}
	sent, err := fp.payloads[0].Compact()
	if err != nil || string(sent) != pv.JSON {
		t.Fatalf("sent=%s preview=%s", sent, pv.JSON)
	}
	if d := r.of(EvAIDone); len(d) != 1 || d[0].(AIDoneDTO).Error != "" || d[0].(AIDoneDTO).Report == nil {
		t.Fatalf("done=%+v", d)
	}
	if pv2, _ := a.PreviewAIPayload(p.ID); !pv2.Cached {
		t.Fatal("preview must report the cache hit after a run")
	}
}

func TestRunAIUsesCache(t *testing.T) {
	a, fp, _, _, p := newAIApp(t)
	run := func(force bool) {
		t.Helper()
		if err := a.RunAI(p.ID, force); err != nil {
			t.Fatal(err)
		}
		a.waitAI()
	}
	run(false)
	run(false)
	if fp.calls() != 1 {
		t.Fatalf("cache miss: calls=%d", fp.calls())
	}
	run(true)
	if fp.calls() != 2 {
		t.Fatalf("force ignored: calls=%d", fp.calls())
	}
	rep, _ := a.GetAIReport()
	if rep == nil || rep.Markdown != "# relatório" || rep.CreatedAt.IsZero() || len(rep.Items) == 0 {
		t.Fatalf("report=%+v", rep)
	}
}

func TestRunAIMergesOpinions(t *testing.T) {
	a, _, _, r, p := newAIApp(t)
	if err := a.RunAI(p.ID, false); err != nil {
		t.Fatal(err)
	}
	a.waitAI()
	root, _ := a.GetChildren(`C:\`, -1)
	other := findNode(t, root, "other")
	if other.Source != "ai" || other.Verdict != "safe" || other.AI == nil || other.AI.Verdict != "safe" || other.AI.Reason != "teste" {
		t.Fatalf("other=%+v", other)
	}
	rep := r.of(EvAIDone)[0].(AIDoneDTO).Report
	var found bool
	for _, it := range rep.Items {
		if strings.EqualFold(it.Path, `C:\other`) {
			found = true // real path, not anonymized
		}
	}
	if !found {
		t.Fatalf("report items=%+v", rep.Items)
	}
	// the rule node is classified by the rule; the AI never overrides it
	proj, _ := a.GetChildren(`C:\`, root[0].ID)
	if nm := findNode(t, proj, "node_modules"); nm.Source != "rule" {
		t.Fatalf("nm=%+v", nm)
	}
	// a plan for the AI-classified node carries the merged class
	a.deps.Planner = cleaner.NewPlanner(a.deps.Protector, &fakeCleanDeps{}, a.IsScanned, nil)
	plan := mustPlan(t, a, "recycle", `C:\other`)
	if plan.Items[0].Source != "ai" || plan.Items[0].Verdict != "safe" {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestAIErrorsAreFriendly(t *testing.T) {
	cases := map[error]string{
		ai.ErrAuth:            "Chave de API inválida",
		ai.ErrModelNotFound:   "Modelo não encontrado",
		ai.ErrRateLimited:     "Limite de requisições atingido",
		ai.ErrTimeout:         "Tempo esgotado",
		ai.ErrNetwork:         "Falha de rede",
		ai.ErrInvalidResponse: "Resposta inválida do provedor",
	}
	for sentinel, want := range cases {
		a, fp, _, r, p := newAIApp(t)
		fp.err = fmt.Errorf("wrapped: %w", sentinel)
		if err := a.TestAIProfile(p.ID); err == nil || err.Error() != want {
			t.Fatalf("TestAIProfile: err=%v want %q", err, want)
		}
		if err := a.RunAI(p.ID, true); err != nil {
			t.Fatal(err)
		}
		a.waitAI()
		d := r.of(EvAIDone)
		if len(d) != 1 || d[0].(AIDoneDTO).Error != want || d[0].(AIDoneDTO).Report != nil {
			t.Fatalf("done=%+v want %q", d, want)
		}
	}
}

func TestTestAIProfileSendsTinyPayload(t *testing.T) {
	a, fp, _, _, p := newAIApp(t)
	if err := a.TestAIProfile(p.ID); err != nil {
		t.Fatal(err)
	}
	if fp.calls() != 1 || len(fp.payloads[0].Classified) != 1 || fp.payloads[0].Classified[0].Path != `C:\example` {
		t.Fatalf("payloads=%+v", fp.payloads)
	}
	if err := a.TestAIProfile("ghost"); err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestRunAIWithoutScansFails(t *testing.T) {
	a, _ := newTestApp(t, fakeScanner{}, nil)
	fp := &fakeProvider{}
	a.deps.NewProvider = fp.factory
	sec := &fakeSecrets{m: map[string]string{}}
	a.deps.SecretGet, a.deps.SecretSet, a.deps.SecretDelete = sec.get, sec.set, sec.del
	p, _ := a.SaveAIProfile(AIProfileDTO{Name: "x", Kind: "anthropic", Model: "m"}, "")
	if err := a.RunAI(p.ID, false); err == nil {
		t.Fatal("expected error with no scanned volume")
	}
}

// usersScanner has two profiles with large unknown folders.
type usersScanner struct{}

func (usersScanner) Scan(_ context.Context, root string, b *tree.Builder, _ *scanner.Progress) error {
	r := b.AddDir(tree.NoNode, root, 0)
	u := b.AddDir(r, "Users", 0)
	for _, n := range []string{"jdoe.CORP", "maria", "Public"} {
		p := b.AddDir(u, n, 0)
		big := b.AddDir(p, "big-"+n, 0)
		b.SetOwnStats(big, 600<<20, 1, 100)
	}
	bk := b.AddDir(r, "backup-maria", 0)
	b.SetOwnStats(bk, 600<<20, 1, 100)
	return nil
}

// The payload anonymizes USERNAME, the USERPROFILE leaf and every profile
// folder under X:\Users, but keeps the shared ones.
func TestBuildAIAnonymizesAllProfiles(t *testing.T) {
	t.Setenv("USERNAME", "jdoe")
	t.Setenv("USERPROFILE", `C:\Users\jdoe.CORP`)
	a, _ := newTestApp(t, usersScanner{}, nil)
	if err := a.StartScan([]string{`C:\`}); err != nil {
		t.Fatal(err)
	}
	a.waitScan()
	b, err := a.buildAI()
	if err != nil {
		t.Fatal(err)
	}
	out := strings.ToLower(string(b.json))
	for _, leak := range []string{"jdoe", "corp", "maria"} {
		if strings.Contains(out, leak) {
			t.Errorf("payload leaks %q: %s", leak, b.json)
		}
	}
	if !strings.Contains(out, `public`) {
		t.Errorf("shared folder should stay: %s", b.json)
	}
}

func TestAIReportItemsCarrySessionNodes(t *testing.T) {
	a, _, _, r, p := newAIApp(t)
	if err := a.RunAI(p.ID, false); err != nil {
		t.Fatal(err)
	}
	a.waitAI()
	ev := r.of(EvAIDone)[0].(AIDoneDTO).Report
	if ev.Profile != "Teste" || ev.Model != "m1" {
		t.Fatalf("event report profile=%q model=%q", ev.Profile, ev.Model)
	}
	// a cited path that is no longer in the scan
	a.aiMu.Lock()
	a.lastReport.Items = append(a.lastReport.Items, AIItemDTO{Path: `C:\gone`, Verdict: "safe", Reason: "x", Confidence: 0.5})
	a.aiMu.Unlock()
	rep, err := a.GetAIReport()
	if err != nil || rep == nil {
		t.Fatalf("rep=%v err=%v", rep, err)
	}
	var other, gone *AIItemDTO
	for i := range rep.Items {
		switch rep.Items[i].Path {
		case `C:\other`:
			other = &rep.Items[i]
		case `C:\gone`:
			gone = &rep.Items[i]
		}
	}
	if other == nil || !other.Found || !other.Selectable || other.Node == nil {
		t.Fatalf("other=%+v", other)
	}
	if n := other.Node; n.Size != 50 || n.Files != 1 || n.Verdict != "safe" || n.Source != "ai" || n.Path != `C:\other` {
		t.Fatalf("other.node=%+v", n)
	}
	if gone == nil || gone.Found || gone.Selectable || gone.Node != nil {
		t.Fatalf("gone=%+v", gone)
	}
}

func TestAIReportKeepItemsAreNotSelectable(t *testing.T) {
	a, fp, _, _, p := newAIApp(t)
	fp.verdict = rules.Keep
	if err := a.RunAI(p.ID, false); err != nil {
		t.Fatal(err)
	}
	a.waitAI()
	rep, _ := a.GetAIReport()
	if rep == nil || len(rep.Items) == 0 {
		t.Fatalf("rep=%+v", rep)
	}
	for _, it := range rep.Items {
		if it.Found && (it.Selectable || it.Node.Verdict != "keep") {
			t.Fatalf("keep item selectable: %+v node=%+v", it, it.Node)
		}
	}
}

// A cited path that is not a folder of the scan says why: a known file, a
// path under a scanned folder, or nothing at all.
func TestAIReportMissingKinds(t *testing.T) {
	a, _, _, _, p := newAIApp(t)
	if err := a.RunAI(p.ID, false); err != nil {
		t.Fatal(err)
	}
	a.waitAI()
	s, err := a.session(`C:\`)
	if err != nil {
		t.Fatal(err)
	}
	s.tree.TopFiles = append(s.tree.TopFiles, tree.FileInfo{Path: `C:\proj\big.iso`, Size: 900})
	a.aiMu.Lock()
	a.lastReport.Items = append(a.lastReport.Items,
		AIItemDTO{Path: `c:\PROJ\big.iso`, Verdict: "safe"},
		AIItemDTO{Path: `C:\proj\small.txt`, Verdict: "safe"},
		AIItemDTO{Path: `D:\nowhere\x`, Verdict: "safe"})
	a.aiMu.Unlock()
	rep, _ := a.GetAIReport()
	want := map[string]string{`C:\other`: "", `c:\PROJ\big.iso`: "file", `C:\proj\small.txt`: "not_folder", `D:\nowhere\x`: "not_found"}
	for _, it := range rep.Items {
		w, ok := want[it.Path]
		if !ok {
			continue
		}
		if it.Missing != w || (w != "" && (it.Found || it.Selectable || it.Node != nil)) {
			t.Errorf("%s: missing=%q found=%v selectable=%v want %q", it.Path, it.Missing, it.Found, it.Selectable, w)
		}
		delete(want, it.Path)
	}
	if len(want) != 0 {
		t.Fatalf("items not in report: %v", want)
	}
}
