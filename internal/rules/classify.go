package rules

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

// InactiveAfter is the age after which a build-artifact's project is inactive.
const InactiveAfter = 90 * 24 * time.Hour

// Classification is the per-node result. A zero Verdict ("") with
// Source == SourceUnknown means no rule or protection applies (UI: "unknown").
type Classification struct {
	RuleID   string
	Verdict  Verdict
	Category Category
	Action   Action
	Inactive bool
	Source   Source
}

var unknownClassification = Classification{Source: SourceUnknown}

type entry struct {
	cls  Classification
	kind ProtectKind // ProtectNone for rule matches
}

// Classifications maps tree nodes to their recorded classification. Nodes
// without an entry inherit from the nearest recorded ancestor, except
// exact-protected ancestors, whose content is classified on its own.
// Read-only after Classify returns.
type Classifications struct {
	m map[tree.NodeID]entry
}

// Get returns the classification of id, inherited when not recorded directly.
func (c *Classifications) Get(t *tree.Tree, id tree.NodeID) Classification {
	if e, ok := c.m[id]; ok {
		return e.cls
	}
	for cur := id; cur >= 0 && int(cur) < len(t.Nodes); {
		cur = t.Nodes[cur].Parent
		if cur == tree.NoNode {
			break
		}
		if e, ok := c.m[cur]; ok && e.kind != ProtectExact {
			return e.cls
		}
	}
	return unknownClassification
}

type compiledRule struct {
	Rule
	path    string // normalized, for path rules
	dirname string // lowercase
	sibling string // lowercase glob
	glob    string // lowercase glob
}

// Classifier applies rules and the protection list. Read-only after
// construction and safe for concurrent use.
type Classifier struct {
	prot      *Protector
	exists    func(path string) bool
	listNames func(dir string) []string
	pathRules []compiledRule
	dirRules  []compiledRule // in load order
	globRules []compiledRule
	byID      map[string]Rule
}

// NewClassifier compiles rs (assumed valid; see Validate). Path rules whose
// variables expand to "" are dropped. exists is kept for callers' symmetry;
// sibling checks use listNames.
func NewClassifier(rs []Rule, p *Protector, env Env, exists func(path string) bool, listNames func(dir string) []string) *Classifier {
	c := &Classifier{prot: p, exists: exists, listNames: listNames, byID: map[string]Rule{}}
	for _, r := range rs {
		if _, dup := c.byID[r.ID]; !dup {
			c.byID[r.ID] = r
		}
		cr := compiledRule{Rule: r}
		switch {
		case r.Match.Path != "":
			n, ok := expandNorm(r.Match.Path, env)
			if !ok {
				continue
			}
			cr.path = n
			c.pathRules = append(c.pathRules, cr)
		case r.Match.Dirname != "":
			cr.dirname = strings.ToLower(r.Match.Dirname)
			cr.sibling = strings.ToLower(r.Match.Sibling)
			c.dirRules = append(c.dirRules, cr)
		case r.Match.Glob != "":
			cr.glob = strings.ToLower(r.Match.Glob)
			c.globRules = append(c.globRules, cr)
		}
	}
	return c
}

// Rule returns the rule with the given id.
func (c *Classifier) Rule(id string) (Rule, bool) {
	r, ok := c.byID[id]
	return r, ok
}

type walkItem struct {
	id        tree.NodeID
	raw, norm string
	protected bool // inside a protected subtree recorded above
}

// Classify walks t top-down and records classifications:
//   - protected nodes get Keep/SourceProtected (keeping the RuleID, Category and
//     Action of a rule that also matches them);
//   - exact-protected nodes are descended normally;
//   - inside a protected subtree only path rules are looked for (so the UI can
//     show their command/explain), and only toward nodes that can hold them;
//   - a rule match consumes the node's subtree, unless it holds a protected
//     path, in which case the walk continues so that path is recorded.
func (c *Classifier) Classify(t *tree.Tree, now time.Time) *Classifications {
	cls := &Classifications{m: map[tree.NodeID]entry{}}
	if t == nil || t.Root == tree.NoNode || len(t.Nodes) == 0 {
		return cls
	}
	cutoff := now.Add(-InactiveAfter).Unix()
	names := map[tree.NodeID][]string{} // listNames cache, keyed by parent node
	rootRaw := t.Nodes[t.Root].Name
	stack := []walkItem{{id: t.Root, raw: rootRaw, norm: NormalizePath(rootRaw)}}
	for len(stack) > 0 {
		it := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		n := &t.Nodes[it.id]

		descend := true
		if it.protected {
			if r, ok := c.matchPath(it.norm); ok {
				cls.m[it.id] = entry{cls: protectedCls(r), kind: ProtectSubtree}
				descend = false
			} else {
				descend = c.pathRuleBelow(it.norm)
			}
		} else {
			kind := c.prot.checkNorm(it.norm)
			r, matched := c.match(t, it.id, it.norm, names)
			switch {
			case kind != ProtectNone:
				pc := Classification{Verdict: Keep, Source: SourceProtected}
				if matched {
					pc = protectedCls(r)
				}
				cls.m[it.id] = entry{cls: pc, kind: kind}
				if kind == ProtectSubtree {
					it.protected = true
					descend = c.pathRuleBelow(it.norm)
				}
			case matched:
				cls.m[it.id] = entry{cls: Classification{
					RuleID: r.ID, Verdict: r.Verdict, Category: r.Category, Action: r.Action,
					Inactive: r.Category == "build-artifact" && inactive(t, it.id, cutoff),
					Source:   SourceRule,
				}}
				descend = c.prot.HasProtectedDescendant(it.norm)
			}
		}
		if !descend {
			continue
		}
		for _, ch := range n.Children {
			name := t.Nodes[ch].Name
			stack = append(stack, walkItem{
				id: ch, raw: joinRaw(it.raw, name), norm: joinNorm(it.norm, name), protected: it.protected,
			})
		}
	}
	return cls
}

func protectedCls(r compiledRule) Classification {
	return Classification{RuleID: r.ID, Verdict: Keep, Category: r.Category, Action: r.Action, Source: SourceProtected}
}

// match finds the winning rule for a directory: path rules first (load order),
// then dirname rules with a matching sibling, then plain dirname rules.
func (c *Classifier) match(t *tree.Tree, id tree.NodeID, norm string, names map[tree.NodeID][]string) (compiledRule, bool) {
	if r, ok := c.matchPath(norm); ok {
		return r, true
	}
	parent := t.Nodes[id].Parent
	if parent == tree.NoNode {
		return compiledRule{}, false
	}
	name := strings.ToLower(t.Nodes[id].Name)
	var plain *compiledRule
	for i := range c.dirRules {
		r := &c.dirRules[i]
		if r.dirname != name {
			continue
		}
		if r.sibling == "" {
			if plain == nil {
				plain = r
			}
			continue
		}
		ls, ok := names[parent]
		if !ok {
			if c.listNames != nil {
				for _, s := range c.listNames(t.Path(parent)) {
					ls = append(ls, strings.ToLower(s))
				}
			}
			names[parent] = ls
		}
		for _, s := range ls {
			if m, err := filepath.Match(r.sibling, s); err == nil && m {
				return *r, true
			}
		}
	}
	if plain != nil {
		return *plain, true
	}
	return compiledRule{}, false
}

func (c *Classifier) matchPath(norm string) (compiledRule, bool) {
	for _, r := range c.pathRules {
		if r.path == norm {
			return r, true
		}
	}
	return compiledRule{}, false
}

// pathRuleBelow reports whether some path rule targets a node strictly below norm.
func (c *Classifier) pathRuleBelow(norm string) bool {
	for _, r := range c.pathRules {
		if r.path != norm && isUnder(r.path, norm) {
			return true
		}
	}
	return false
}

// inactive: the most recent mtime of the parent directory excluding the
// artifact (parent's own files and its other children's subtrees) is older
// than cutoff. A parent with no other content is not inactive.
func inactive(t *tree.Tree, id tree.NodeID, cutoff int64) bool {
	p := t.Nodes[id].Parent
	if p == tree.NoNode {
		return false
	}
	ref := t.Nodes[p].OwnLatest
	for _, ch := range t.Nodes[p].Children {
		if ch != id && t.Nodes[ch].TreeLatest > ref {
			ref = t.Nodes[ch].TreeLatest
		}
	}
	return ref != 0 && ref < cutoff
}

func joinRaw(dir, name string) string {
	if strings.HasSuffix(dir, `\`) {
		return dir + name
	}
	return dir + `\` + name
}

// ClassifyFile classifies a single file with the glob rules. Protected paths
// return a Keep/SourceProtected classification. ok is false when nothing applies.
func (c *Classifier) ClassifyFile(path string, modTime time.Time, now time.Time) (Classification, bool) {
	norm := NormalizePath(path)
	base := norm
	if i := strings.LastIndexByte(norm, '\\'); i >= 0 {
		base = norm[i+1:]
	}
	var hit *compiledRule
	for i := range c.globRules {
		r := &c.globRules[i]
		if m, err := filepath.Match(r.glob, base); err != nil || !m {
			continue
		}
		if d := r.Match.OlderThanDays; d > 0 && !modTime.Before(now.Add(-time.Duration(d)*24*time.Hour)) {
			continue
		}
		hit = r
		break
	}
	if c.prot.checkNorm(norm) != ProtectNone {
		if hit != nil {
			return protectedCls(*hit), true
		}
		return Classification{Verdict: Keep, Source: SourceProtected}, true
	}
	if hit == nil {
		return unknownClassification, false
	}
	return Classification{RuleID: hit.ID, Verdict: hit.Verdict, Category: hit.Category, Action: hit.Action, Source: SourceRule}, true
}

// Candidates returns unknown nodes (no own or inherited classification) with
// TotalSize >= minBytes, excluding the root and links, sorted by TotalSize
// descending (ties in tree order). Nested unknown nodes are all returned.
func Candidates(t *tree.Tree, cls *Classifications, minBytes int64) []tree.NodeID {
	var out []tree.NodeID
	for i := range t.Nodes {
		id := tree.NodeID(i)
		n := &t.Nodes[i]
		if id == t.Root || n.Flags&tree.FlagLink != 0 || n.TotalSize < minBytes {
			continue
		}
		if cls.Get(t, id).Source != SourceUnknown {
			continue
		}
		out = append(out, id)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return t.Nodes[out[i]].TotalSize > t.Nodes[out[j]].TotalSize
	})
	return out
}
