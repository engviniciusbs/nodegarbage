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

// minSubstringName is the shortest user name that is replaced as a substring
// and guarded. Known limitation: shorter names (e.g. "dev") are replaced only
// as a whole path component, because substring replacement would mangle
// unrelated paths such as C:\devtools and a guard would drop too much. Such a
// name embedded in a longer component (e.g. "dev-backup") is NOT anonymized.
const minSubstringName = 4

// Anonymizer removes the real Windows user name and user-supplied sensitive
// terms from paths.
type Anonymizer struct {
	userName string
	extra    *regexp.Regexp // nil when there are no terms
	userRe   *regexp.Regexp // substring match; nil for short names
}

// NewAnonymizer: userName (case-insensitive) becomes "<user>" anywhere in the
// path when it has 4+ characters, otherwise only as a whole path component;
// each non-empty extra term is replaced as a case-insensitive substring by
// "<redacted>".
func NewAnonymizer(userName string, extra []string) *Anonymizer {
	a := &Anonymizer{userName: strings.TrimSpace(userName)}
	var terms []string
	for _, t := range extra {
		if t = strings.TrimSpace(t); t != "" {
			terms = append(terms, t)
		}
	}
	if len(a.userName) >= minSubstringName {
		a.userRe = regexp.MustCompile("(?i)" + regexp.QuoteMeta(a.userName))
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
	if a.extra != nil {
		path = a.extra.ReplaceAllLiteralString(path, redactedPlaceholder)
	}
	if a.userRe != nil {
		return a.userRe.ReplaceAllLiteralString(path, userPlaceholder)
	}
	if a.userName == "" {
		return path
	}
	comps := strings.Split(path, `\`)
	for i, c := range comps {
		if strings.EqualFold(c, a.userName) {
			comps[i] = userPlaceholder
		}
	}
	return strings.Join(comps, `\`)
}

// Leaks reports whether s still contains the user name (case-insensitive).
// Active only for names of 4+ characters.
func (a *Anonymizer) Leaks(s string) bool {
	return a.userRe != nil && a.userRe.MatchString(s)
}
