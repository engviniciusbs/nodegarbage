package app

import (
	"context"
	"testing"

	"github.com/engviniciusbs/nodegarbage/internal/ai"
	"github.com/engviniciusbs/nodegarbage/internal/cleaner"
	"github.com/engviniciusbs/nodegarbage/internal/rules"
	"github.com/engviniciusbs/nodegarbage/internal/scanner"
	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

func TestSettingsDefaultsAndRoundTrip(t *testing.T) {
	a, _ := newTestApp(t, fakeScanner{}, nil)
	a.deps.UserRulesDir = `C:\data\rules.d`
	a.deps.RuleProblems = []rules.LoadProblem{{File: "x.yaml", RuleID: "r1", Err: "bad"}}
	s, err := a.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.AIMinBytes != 500<<20 || s.Workers != 0 || s.RedactTerms == nil || s.ExtraProtected == nil ||
		s.UserRulesDir != `C:\data\rules.d` || len(s.RuleProblems) != 1 || s.RuleProblems[0].Error != "bad" {
		t.Fatalf("defaults=%+v", s)
	}
	in := SettingsDTO{RedactTerms: []string{" secret ", "", "SECRET"}, ExtraProtected: []string{`D:\keep`},
		AIMinBytes: 123, Workers: 3, UserRulesDir: "ignored", RuleProblems: []RuleProblemDTO{{File: "ignored"}}}
	if err := a.SaveSettings(in); err != nil {
		t.Fatal(err)
	}
	s, _ = a.GetSettings()
	if len(s.RedactTerms) != 1 || s.RedactTerms[0] != "secret" || s.ExtraProtected[0] != `D:\keep` ||
		s.AIMinBytes != 123 || s.Workers != 3 || s.UserRulesDir != `C:\data\rules.d` || len(s.RuleProblems) != 1 {
		t.Fatalf("saved=%+v", s)
	}
	raw, _, _ := a.deps.Store.GetSetting(context.Background(), "settings")
	if string(raw) == "" || contains(string(raw), "ignored") || contains(string(raw), "ruleProblems") {
		t.Fatalf("derived fields persisted: %s", raw)
	}
	for _, bad := range []SettingsDTO{{AIMinBytes: -1}, {Workers: -1}} {
		if err := a.SaveSettings(bad); err == nil {
			t.Fatalf("expected error for %+v", bad)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestSaveSettingsAppliesExtraProtectedImmediately(t *testing.T) {
	a, _, _ := newCleanApp(t)
	if p := mustPlan(t, a, "recycle", `C:\other`); len(p.Items) != 1 {
		t.Fatalf("precondition: %+v", p)
	}
	if err := a.SaveSettings(SettingsDTO{ExtraProtected: []string{`C:\other`}}); err != nil {
		t.Fatal(err)
	}
	if p := mustPlan(t, a, "recycle", `C:\other`); len(p.Items) != 0 || len(p.Refused) != 1 || p.Refused[0].Reason != cleaner.ReasonKeep {
		t.Fatalf("plan after SetExtra: %+v", p)
	}
	root, _ := a.GetChildren(`C:\`, -1)
	if o := findNode(t, root, "other"); o.Source != "protected" || o.Verdict != "keep" {
		t.Fatalf("sessions not reclassified: %+v", o)
	}
	if err := a.SaveSettings(SettingsDTO{}); err != nil {
		t.Fatal(err)
	}
	root, _ = a.GetChildren(`C:\`, -1)
	if o := findNode(t, root, "other"); o.Source != "unknown" {
		t.Fatalf("protection not lifted: %+v", o)
	}
}

func TestExtraProtectedAppliedAtStartup(t *testing.T) {
	a, _ := newTestApp(t, fakeScanner{}, nil)
	if err := a.SaveSettings(SettingsDTO{ExtraProtected: []string{`D:\x`}}); err != nil {
		t.Fatal(err)
	}
	b, _ := newTestApp(t, fakeScanner{}, a.deps.Store)
	if b.deps.Protector.Check(`D:\x\y`) != rules.ProtectNone {
		t.Fatal("precondition")
	}
	b.Startup(nil)
	if b.deps.Protector.Check(`D:\x\y`) != rules.ProtectSubtree {
		t.Fatal("extra protected paths not applied at startup")
	}
}

func TestWorkersPassedToScanner(t *testing.T) {
	a, _ := newTestApp(t, fakeScanner{}, nil)
	var got []int
	a.deps.NewScanner = func(_ string, w int) scanner.Scanner {
		got = append(got, w)
		return fakeScanner{}
	}
	if err := a.SaveSettings(SettingsDTO{Workers: 7}); err != nil {
		t.Fatal(err)
	}
	if err := a.StartScan([]string{`C:\`}); err != nil {
		t.Fatal(err)
	}
	a.waitScan()
	if len(got) != 1 || got[0] != 7 {
		t.Fatalf("workers=%v", got)
	}
}

// R16: a classified node nested under an already counted one (here an AI
// opinion with a different verdict on a node inside a rule node) never counts twice.
func TestSummaryNoDoubleCountWithNestedAIOpinion(t *testing.T) {
	a := scanned(t)
	key := volKey(`C:\`)
	a.updateSession(key, func(old *session) *session {
		inner, ok := old.tree.Find(`C:\proj\node_modules\node_modules`)
		if !ok {
			t.Fatal("inner node missing")
		}
		n := *old
		n.aiOpinions = map[tree.NodeID]ai.Resolved{inner: {RealPath: `C:\proj\node_modules\node_modules`, Verdict: rules.Keep}}
		return &n
	})
	s, _ := a.GetSummary(`C:\`)
	if s.SafeBytes != 1500 || s.KeepBytes != 0 || s.CautionBytes != 0 {
		t.Fatalf("summary=%+v", s)
	}
	for _, c := range s.ByCategory {
		if c.Count != 1 {
			t.Fatalf("cats=%+v", s.ByCategory)
		}
	}
}

func TestSummaryCountsAIOpinionOnUnknownNode(t *testing.T) {
	a := scanned(t)
	a.updateSession(volKey(`C:\`), func(old *session) *session {
		o, _ := old.tree.Find(`C:\other`)
		n := *old
		n.aiOpinions = map[tree.NodeID]ai.Resolved{o: {RealPath: `C:\other`, Verdict: rules.Safe}}
		return &n
	})
	s, _ := a.GetSummary(`C:\`)
	if s.SafeBytes != 1550 {
		t.Fatalf("summary=%+v", s)
	}
}

func TestListVolumesAfterCleanRemovedNodeNotCounted(t *testing.T) {
	a, _, _ := newCleanApp(t)
	p := mustPlan(t, a, "recycle", `C:\proj\node_modules`)
	if err := a.ExecuteClean(p.ID, p.Token); err != nil {
		t.Fatal(err)
	}
	a.waitClean()
	if s, _ := a.GetSummary(`C:\`); s.SafeBytes != 0 || len(s.ByCategory) != 0 {
		t.Fatalf("removed node still summarized: %+v", s)
	}
}
