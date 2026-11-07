package ai

import (
	"testing"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
)

func TestResolveDropsHallucinated(t *testing.T) {
	pm := PathMap{rules.NormalizePath(`C:\Users\<user>\a`): `C:\Users\vini\a`, rules.NormalizePath(`C:\b`): `C:\b`}
	a := Analysis{Items: []ItemVerdict{
		{Path: `c:/users/<USER>/a/`, Verdict: "safe", Confidence: 2},
		{Path: `C:\invented`, Verdict: "safe"},
		{Path: `C:\b`, Verdict: "delete"},
		{Path: `C:\users\<user>\a`, Verdict: "keep"}, // duplicate: first wins
		{Path: `C:\b`, Verdict: "caution", Confidence: -1},
	}}
	got := Resolve(a, pm)
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	if got[0].RealPath != `C:\Users\vini\a` || got[0].Verdict != rules.Safe || got[0].Confidence != 1 {
		t.Errorf("%+v", got[0])
	}
	if got[1].RealPath != `C:\b` || got[1].Verdict != rules.Caution || got[1].Confidence != 0 {
		t.Errorf("%+v", got[1])
	}
}

func TestMerge(t *testing.T) {
	cases := []struct {
		name       string
		base       rules.Classification
		ai         rules.Verdict
		wantV      rules.Verdict
		wantSource rules.Source
	}{
		{"protected+safe", rules.Classification{Verdict: rules.Keep, Source: rules.SourceProtected}, rules.Safe, rules.Keep, rules.SourceProtected},
		{"unknown+safe", rules.Classification{Source: rules.SourceUnknown}, rules.Safe, rules.Safe, rules.SourceAI},
		{"rule safe+caution", rules.Classification{Verdict: rules.Safe, Source: rules.SourceRule}, rules.Caution, rules.Caution, rules.SourceRule},
		{"rule caution+safe", rules.Classification{Verdict: rules.Caution, Source: rules.SourceRule}, rules.Safe, rules.Caution, rules.SourceRule},
	}
	for _, c := range cases {
		got := Merge(c.base, Resolved{Verdict: c.ai})
		if got.Verdict != c.wantV || got.Source != c.wantSource {
			t.Errorf("%s: %+v", c.name, got)
		}
	}
	p := rules.Classification{Verdict: rules.Keep, Source: rules.SourceProtected, RuleID: "x"}
	if Merge(p, Resolved{Verdict: rules.Safe}) != p {
		t.Error("protected must be intact")
	}
}

func TestCacheKeyStable(t *testing.T) {
	a := CacheKey([]byte("p"), KindAnthropic, "m")
	if a != CacheKey([]byte("p"), KindAnthropic, "m") || len(a) != 64 {
		t.Error("unstable")
	}
	if a == CacheKey([]byte("p"), KindAnthropic, "m2") || a == CacheKey([]byte("p"), KindOpenAICompat, "m") || a == CacheKey([]byte("q"), KindAnthropic, "m") {
		t.Error("collision")
	}
}
