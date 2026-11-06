// Package ai builds the anonymized, size-bounded payload sent to an LLM.
package ai

import (
	"regexp"
	"sort"
	"strings"
)

const (
	userPlaceholder     = "<user>"
	redactedPlaceholder = "<redacted>"
)

// Anonymizer removes the real Windows user name and user-supplied sensitive
// terms from paths.
type Anonymizer struct {
	userName string
	extra    *regexp.Regexp // nil when there are no terms
}

// NewAnonymizer: userName is replaced (case-insensitively) when it is a whole
// path component (this covers C:\Users\<name>); each non-empty extra term is
// replaced as a case-insensitive substring by "<redacted>".
func NewAnonymizer(userName string, extra []string) *Anonymizer {
	a := &Anonymizer{userName: strings.TrimSpace(userName)}
	var terms []string
	for _, t := range extra {
		if t = strings.TrimSpace(t); t != "" {
			terms = append(terms, t)
		}
	}
	if len(terms) > 0 {
		sort.SliceStable(terms, func(i, j int) bool { return len(terms[i]) > len(terms[j]) })
		quoted := make([]string, len(terms))
		for i, t := range terms {
			quoted[i] = regexp.QuoteMeta(t)
		}
		a.extra = regexp.MustCompile("(?i)" + strings.Join(quoted, "|"))
	}
	return a
}

// Apply returns path with the user name and extra terms replaced.
func (a *Anonymizer) Apply(path string) string {
	comps := strings.Split(path, `\`)
	for i, c := range comps {
		if a.userName != "" && strings.EqualFold(c, a.userName) {
			comps[i] = userPlaceholder
			continue
		}
		if a.extra != nil {
			comps[i] = a.extra.ReplaceAllLiteralString(c, redactedPlaceholder)
		}
	}
	return strings.Join(comps, `\`)
}
