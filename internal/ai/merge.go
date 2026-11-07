package ai

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
)

// Resolved is an AI verdict mapped back to a real path.
type Resolved struct {
	RealPath   string
	Verdict    rules.Verdict
	Reason     string
	Confidence float64
}

// Resolve maps the model's items to real paths. Paths outside pm (invented by
// the model) and verdicts outside safe|caution|keep are dropped; duplicates
// keep the first.
func Resolve(a Analysis, pm PathMap) []Resolved {
	var out []Resolved
	seen := map[string]bool{}
	for _, it := range a.Items {
		switch it.Verdict {
		case rules.Safe, rules.Caution, rules.Keep:
		default:
			continue
		}
		real, ok := pm[rules.NormalizePath(it.Path)]
		if !ok || real == "" {
			continue
		}
		k := rules.NormalizePath(real)
		if seen[k] {
			continue
		}
		seen[k] = true
		c := it.Confidence
		if !(c >= 0) { // also catches NaN
			c = 0
		} else if c > 1 {
			c = 1
		}
		out = append(out, Resolved{RealPath: real, Verdict: it.Verdict, Reason: it.Reason, Confidence: c})
	}
	return out
}

// Merge combines a rule classification with an AI verdict, conservatively.
func Merge(base rules.Classification, r Resolved) rules.Classification {
	switch base.Source {
	case rules.SourceUnknown:
		base.Verdict = r.Verdict
		base.Source = rules.SourceAI
	case rules.SourceRule:
		base.Verdict = rules.MostConservative(base.Verdict, r.Verdict)
	}
	return base // protected (and anything else) stays intact
}

// CacheKey identifies an analysis request: hex sha256 of payload, kind and model.
func CacheKey(payloadJSON []byte, kind Kind, model string) string {
	h := sha256.New()
	h.Write(payloadJSON)
	h.Write([]byte{0})
	h.Write([]byte(kind))
	h.Write([]byte{0})
	h.Write([]byte(model))
	return hex.EncodeToString(h.Sum(nil))
}
