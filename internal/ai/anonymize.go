// Package ai builds the anonymized, size-bounded payload sent to an LLM.
package ai

import (
	"regexp"
	"sort"
	"strings"

	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

const (
	userPlaceholder     = "<user>"
	redactedPlaceholder = "<redacted>"
)

// minSubstringName is the shortest user name that is replaced as a substring
// and guarded. Known limitation: shorter names (e.g. "dev") are replaced only
// as a whole path component, because substring replacement would mangle
// unrelated paths such as C:\devtools and a guard would drop too much. Such a
// name embedded in a longer component (e.g. "dev-backup") is NOT anonymized,
// except as the segment right after X:\Users\ (always replaced, see Apply).
const minSubstringName = 4

// sharedProfiles are the well-known folders under X:\Users that belong to no
// user. They are never anonymized and never taken as user names.
var sharedProfiles = map[string]bool{
	"public": true, "default": true, "all users": true, "default user": true,
}

// usersSegment matches `X:\Users\<segment>` at the start of a path.
var usersSegment = regexp.MustCompile(`(?i)^([a-z]:\\users\\)([^\\]+)`)

// Anonymizer removes Windows user names and user-supplied sensitive terms
// from paths.
type Anonymizer struct {
	short  map[string]bool // lowercase names < minSubstringName: whole component only
	extra  *regexp.Regexp  // nil when there are no terms
	userRe *regexp.Regexp  // substring match of every long name; nil if none
}

// NewAnonymizer is NewUserAnonymizer with a single user name.
func NewAnonymizer(userName string, extra []string) *Anonymizer {
	return NewUserAnonymizer([]string{userName}, extra)
}

// NewUserAnonymizer: each user name (case-insensitive; typically USERNAME,
// the leaf of USERPROFILE and the profile folders found under X:\Users)
// becomes "<user>" anywhere in the path when it has 4+ characters, otherwise
// only as a whole path component. Shared profile folders (Public, Default,
// All Users, Default User) are ignored as names. Each non-empty extra term is
// replaced as a case-insensitive substring by "<redacted>".
func NewUserAnonymizer(userNames []string, extra []string) *Anonymizer {
	a := &Anonymizer{short: map[string]bool{}}
	var long []string
	seen := map[string]bool{}
	for _, n := range userNames {
		n = strings.TrimSpace(n)
		k := strings.ToLower(n)
		if n == "" || seen[k] || sharedProfiles[k] {
			continue
		}
		seen[k] = true
		if len(n) >= minSubstringName {
			long = append(long, n)
		} else {
			a.short[k] = true
		}
	}
	a.userRe = alternation(long)
	var terms []string
	for _, t := range extra {
		if t = strings.TrimSpace(t); t != "" {
			terms = append(terms, t)
		}
	}
	a.extra = alternation(terms)
	return a
}

// alternation builds a case-insensitive regexp matching any of the literal
// terms, longest first (so "jdoe.CORP" wins over "jdoe"); nil when empty.
func alternation(terms []string) *regexp.Regexp {
	if len(terms) == 0 {
		return nil
	}
	sorted := append([]string(nil), terms...)
	sort.SliceStable(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })
	quoted := make([]string, len(sorted))
	for i, t := range sorted {
		quoted[i] = regexp.QuoteMeta(t)
	}
	return regexp.MustCompile("(?i)" + strings.Join(quoted, "|"))
}

// Apply returns path with the extra terms and user names replaced. The
// segment right after a leading X:\Users\ is always replaced by "<user>"
// (whoever it belongs to), unless it is a shared profile folder.
func (a *Anonymizer) Apply(path string) string {
	if a.extra != nil {
		path = a.extra.ReplaceAllLiteralString(path, redactedPlaceholder)
	}
	if a.userRe != nil {
		path = a.userRe.ReplaceAllLiteralString(path, userPlaceholder)
	}
	if len(a.short) > 0 {
		comps := strings.Split(path, `\`)
		for i, c := range comps {
			if a.short[strings.ToLower(c)] {
				comps[i] = userPlaceholder
			}
		}
		path = strings.Join(comps, `\`)
	}
	if m := usersSegment.FindStringSubmatchIndex(path); m != nil {
		seg := path[m[4]:m[5]]
		if !sharedProfiles[strings.ToLower(seg)] {
			path = path[:m[4]] + userPlaceholder + path[m[5]:]
		}
	}
	return path
}

// Leaks reports whether s still contains a user name (case-insensitive).
// Active only for names of 4+ characters.
func (a *Anonymizer) Leaks(s string) bool {
	return a.userRe != nil && a.userRe.MatchString(s)
}

// UserNamesFromTrees returns the sorted, distinct names of the profile
// folders directly under <root>\Users in each tree, without the shared ones.
func UserNamesFromTrees(trees ...*tree.Tree) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range trees {
		if t == nil || t.Root < 0 || int(t.Root) >= len(t.Nodes) {
			continue
		}
		for _, u := range t.Nodes[t.Root].Children {
			if !strings.EqualFold(t.Nodes[u].Name, "Users") {
				continue
			}
			for _, c := range t.Nodes[u].Children {
				n := t.Nodes[c].Name
				k := strings.ToLower(n)
				if n == "" || sharedProfiles[k] || seen[k] {
					continue
				}
				seen[k] = true
				out = append(out, n)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}
