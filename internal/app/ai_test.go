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
