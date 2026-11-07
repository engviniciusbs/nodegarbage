package rules

import (
	"strings"
	"testing"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

type dirSpec struct {
	rel    string // relative to root, `\`-separated
	size   int64
	latest int64
	flags  tree.Flags
}

// buildTree creates a tree rooted at root; parents must be listed before children.
func buildTree(t *testing.T, root string, dirs ...dirSpec) *tree.Tree {
	t.Helper()
	b := tree.NewBuilder(root, 10)
	ids := map[string]tree.NodeID{"": b.AddDir(tree.NoNode, root, 0)}
	for _, d := range dirs {
		parent := ""
		name := d.rel
		if i := strings.LastIndex(d.rel, `\`); i >= 0 {
			parent, name = d.rel[:i], d.rel[i+1:]
		}
		pid, ok := ids[parent]
		if !ok {
			t.Fatalf("parent of %q not defined", d.rel)
		}
		id := b.AddDir(pid, name, d.flags)
		b.SetOwnStats(id, d.size, 1, d.latest)
		ids[d.rel] = id
	}
	return b.Finish()
}

func find(t *testing.T, tr *tree.Tree, path string) tree.NodeID {
	t.Helper()
	id, ok := tr.Find(path)
	if !ok {
		t.Fatalf("node %q not found", path)
	}
	return id
}

// fakeFS answers listNames from a map of lowercase dir -> names.
type fakeFS map[string][]string

func (f fakeFS) listNames(dir string) []string { return f[NormalizePath(dir)] }
func (f fakeFS) exists(path string) bool {
	n := NormalizePath(path)
	i := strings.LastIndex(n, `\`)
	for _, name := range f[n[:i]] {
		if strings.EqualFold(name, n[i+1:]) {
			return true
		}
	}
	return false
}

func newTestClassifier(rs []Rule, fs fakeFS) *Classifier {
	return NewClassifier(rs, newTestProtector(), fakeEnv, fs.exists, fs.listNames)
}

var nodeModulesRule = Rule{
	ID: "node.node_modules", Category: "build-artifact",
	Match: Match{Dirname: "node_modules", Sibling: "package.json"}, Verdict: Safe, Action: ActionDeleteDir,
}

const day = int64(24 * 3600)

var testNow = time.Unix(2_000_000_000, 0)

func TestNodeModulesNeedsSibling(t *testing.T) {
	tr := buildTree(t, `D:\`,
		dirSpec{rel: `proj`}, dirSpec{rel: `proj\node_modules`, size: 10},
		dirSpec{rel: `other`}, dirSpec{rel: `other\node_modules`, size: 10})
	fs := fakeFS{`d:\proj`: {"Package.JSON", "node_modules"}, `d:\other`: {"node_modules", "readme.md"}}
	cls := newTestClassifier([]Rule{nodeModulesRule}, fs).Classify(tr, testNow)

	got := cls.Get(tr, find(t, tr, `D:\proj\node_modules`))
	if got.RuleID != "node.node_modules" || got.Source != SourceRule || got.Verdict != Safe || got.Action != ActionDeleteDir {
		t.Fatalf("with sibling: %+v", got)
	}
	got = cls.Get(tr, find(t, tr, `D:\other\node_modules`))
	if got.Source != SourceUnknown || got.RuleID != "" || got.Verdict != "" {
		t.Fatalf("without sibling: %+v", got)
	}
}

func TestSiblingGlob(t *testing.T) {
	r := Rule{ID: "dotnet.bin", Category: "build-artifact", Match: Match{Dirname: "BIN", Sibling: "*.CSPROJ"}, Verdict: Safe, Action: ActionDeleteDir}
	tr := buildTree(t, `D:\`, dirSpec{rel: `app`}, dirSpec{rel: `app\bin`})
	fs := fakeFS{`d:\app`: {"App.csproj", "bin"}}
	cls := newTestClassifier([]Rule{r}, fs).Classify(tr, testNow)
	if got := cls.Get(tr, find(t, tr, `D:\app\bin`)); got.RuleID != "dotnet.bin" {
		t.Fatalf("%+v", got)
	}
}

func TestPathRuleBeatsDirname(t *testing.T) {
	dirRule := Rule{ID: "generic.cache", Category: "dev-cache", Match: Match{Dirname: "cache"}, Verdict: Caution, Action: ActionDeleteDir}
	pathRule := Rule{ID: "x.cache", Category: "dev-cache", Match: Match{Path: `%LOCALAPPDATA%\x\cache`}, Verdict: Safe, Action: ActionDeleteContents}
	tr := buildTree(t, `C:\`,
		dirSpec{rel: `Users`}, dirSpec{rel: `Users\dev`}, dirSpec{rel: `Users\dev\AppData`},
		dirSpec{rel: `Users\dev\AppData\Local`}, dirSpec{rel: `Users\dev\AppData\Local\X`},
		dirSpec{rel: `Users\dev\AppData\Local\X\Cache`})
	// dirname rule listed first: load order must not beat path precedence.
	cls := newTestClassifier([]Rule{dirRule, pathRule}, fakeFS{}).Classify(tr, testNow)
	got := cls.Get(tr, find(t, tr, `C:\Users\dev\AppData\Local\x\cache`))
	if got.RuleID != "x.cache" || got.Verdict != Safe || got.Action != ActionDeleteContents {
		t.Fatalf("%+v", got)
	}
}

func TestSiblingMoreSpecific(t *testing.T) {
	plain := Rule{ID: "generic.target", Category: "build-artifact", Match: Match{Dirname: "target"}, Verdict: Caution, Action: ActionDeleteDir}
	rust := Rule{ID: "rust.target", Category: "build-artifact", Match: Match{Dirname: "target", Sibling: "Cargo.toml"}, Verdict: Safe, Action: ActionDeleteDir}
	tr := buildTree(t, `D:\`, dirSpec{rel: `r`}, dirSpec{rel: `r\target`}, dirSpec{rel: `j`}, dirSpec{rel: `j\target`})
	fs := fakeFS{`d:\r`: {"Cargo.toml", "target"}, `d:\j`: {"pom.xml", "target"}}
	cls := newTestClassifier([]Rule{plain, rust}, fs).Classify(tr, testNow)
	if got := cls.Get(tr, find(t, tr, `D:\r\target`)); got.RuleID != "rust.target" {
		t.Fatalf("rust: %+v", got)
	}
	if got := cls.Get(tr, find(t, tr, `D:\j\target`)); got.RuleID != "generic.target" {
		t.Fatalf("plain: %+v", got)
	}
}

func TestLoadOrderTieBreak(t *testing.T) {
	a := Rule{ID: "a", Category: "temp", Match: Match{Dirname: "tmp"}, Verdict: Safe, Action: ActionDeleteDir}
	b := Rule{ID: "b", Category: "temp", Match: Match{Dirname: "tmp"}, Verdict: Caution, Action: ActionDeleteDir}
	tr := buildTree(t, `D:\`, dirSpec{rel: `tmp`})
	cls := newTestClassifier([]Rule{a, b}, fakeFS{}).Classify(tr, testNow)
	if got := cls.Get(tr, find(t, tr, `D:\tmp`)); got.RuleID != "a" {
		t.Fatalf("%+v", got)
	}
}

func TestConsumedSubtree(t *testing.T) {
	tr := buildTree(t, `D:\`, dirSpec{rel: `p`}, dirSpec{rel: `p\node_modules`},
		dirSpec{rel: `p\node_modules\a`}, dirSpec{rel: `p\node_modules\a\node_modules`})
	fs := fakeFS{`d:\p`: {"package.json"}, `d:\p\node_modules\a`: {"package.json"}}
	cls := newTestClassifier([]Rule{nodeModulesRule}, fs).Classify(tr, testNow)
	outer := find(t, tr, `D:\p\node_modules`)
	inner := find(t, tr, `D:\p\node_modules\a\node_modules`)
	if _, ok := cls.m[outer]; !ok {
		t.Fatal("outer not recorded")
	}
	if _, ok := cls.m[inner]; ok {
		t.Fatal("inner must not be recorded (consumed)")
	}
	if len(cls.m) != 2 { // the exact-protected root D:\ and the outer node_modules
		t.Fatalf("map has %d entries", len(cls.m))
	}
	if got, want := cls.Get(tr, inner), cls.Get(tr, outer); got != want {
		t.Fatalf("inner %+v != outer %+v", got, want)
	}
}

func TestProtectedOverridesRule(t *testing.T) {
	r := Rule{ID: "win.sys32", Category: "system", Match: Match{Path: `%SystemRoot%\System32`}, Verdict: Safe, Action: ActionManual, Command: "cleanmgr"}
	tr := buildTree(t, `C:\`, dirSpec{rel: `Windows`}, dirSpec{rel: `Windows\System32`},
		dirSpec{rel: `Windows\System32\drivers`}, dirSpec{rel: `Windows\Fonts`})
	cls := newTestClassifier([]Rule{r}, fakeFS{}).Classify(tr, testNow)
	got := cls.Get(tr, find(t, tr, `C:\Windows\System32`))
	if got.Verdict != Keep || got.Source != SourceProtected || got.RuleID != "win.sys32" || got.Action != ActionManual || got.Category != "system" {
		t.Fatalf("%+v", got)
	}
	if got := cls.Get(tr, find(t, tr, `C:\Windows\System32\drivers`)); got.Verdict != Keep || got.Source != SourceProtected {
		t.Fatalf("descendant: %+v", got)
	}
	if got := cls.Get(tr, find(t, tr, `C:\Windows\Fonts`)); got.Verdict != Keep || got.Source != SourceProtected || got.RuleID != "" {
		t.Fatalf("fonts: %+v", got)
	}
}

func TestProtectedTreeRoot(t *testing.T) {
	// Exact-protected nodes are not inherited; their children are classified normally.
	tr := buildTree(t, `C:\`, dirSpec{rel: `Users`}, dirSpec{rel: `Users\dev`}, dirSpec{rel: `Users\dev\Documents`},
		dirSpec{rel: `Users\dev\Documents\stuff`}, dirSpec{rel: `proj`}, dirSpec{rel: `proj\.git`}, dirSpec{rel: `proj\.git\objects`})
	cls := newTestClassifier(nil, fakeFS{}).Classify(tr, testNow)
	if got := cls.Get(tr, tr.Root); got.Source != SourceProtected || got.Verdict != Keep {
		t.Fatalf("root: %+v", got)
	}
	if got := cls.Get(tr, find(t, tr, `C:\Users\dev\Documents`)); got.Source != SourceProtected {
		t.Fatalf("documents: %+v", got)
	}
	if got := cls.Get(tr, find(t, tr, `C:\Users\dev\Documents\stuff`)); got.Source != SourceUnknown {
		t.Fatalf("stuff: %+v", got)
	}
	if got := cls.Get(tr, find(t, tr, `C:\proj\.git\objects`)); got.Source != SourceProtected {
		t.Fatalf(".git: %+v", got)
	}
}

func TestRuleMatchWithProtectedDescendant(t *testing.T) {
	// A rule that would consume a directory holding a protected path must not hide it.
	r := Rule{ID: "local", Category: "temp", Match: Match{Path: `%LOCALAPPDATA%\Stuff`}, Verdict: Safe, Action: ActionDeleteDir}
	tr := buildTree(t, `C:\`, dirSpec{rel: `Users`}, dirSpec{rel: `Users\dev`}, dirSpec{rel: `Users\dev\AppData`},
		dirSpec{rel: `Users\dev\AppData\Local`}, dirSpec{rel: `Users\dev\AppData\Local\Stuff`},
		dirSpec{rel: `Users\dev\AppData\Local\Stuff\vault`}, dirSpec{rel: `Users\dev\AppData\Local\Stuff\junk`})
	p := NewProtector(fakeEnv, []string{`C:\`}, []string{`%LOCALAPPDATA%\Stuff\vault`})
	cls := NewClassifier([]Rule{r}, p, fakeEnv, fakeFS{}.exists, fakeFS{}.listNames).Classify(tr, testNow)
	if got := cls.Get(tr, find(t, tr, `C:\Users\dev\AppData\Local\Stuff\vault`)); got.Source != SourceProtected || got.Verdict != Keep {
		t.Fatalf("vault: %+v", got)
	}
	if got := cls.Get(tr, find(t, tr, `C:\Users\dev\AppData\Local\Stuff\junk`)); got.RuleID != "local" {
		t.Fatalf("junk: %+v", got)
	}
}

func TestInactiveProject(t *testing.T) {
	for _, c := range []struct {
		age  int64
		want bool
	}{{91, true}, {89, false}} {
		now := testNow.Unix()
		tr := buildTree(t, `D:\`,
			dirSpec{rel: `p`, latest: now - c.age*day},
			dirSpec{rel: `p\src`, latest: now - (c.age+5)*day},
			dirSpec{rel: `p\node_modules`, latest: now}) // the artifact itself is excluded
		fs := fakeFS{`d:\p`: {"package.json"}}
		cls := newTestClassifier([]Rule{nodeModulesRule}, fs).Classify(tr, testNow)
		if got := cls.Get(tr, find(t, tr, `D:\p\node_modules`)); got.Inactive != c.want {
			t.Errorf("age %d: inactive = %v, want %v", c.age, got.Inactive, c.want)
		}
	}
	// Sibling subtree activity counts.
	now := testNow.Unix()
	tr := buildTree(t, `D:\`, dirSpec{rel: `p`, latest: now - 200*day}, dirSpec{rel: `p\src`},
		dirSpec{rel: `p\src\deep`, latest: now - day}, dirSpec{rel: `p\node_modules`})
	cls := newTestClassifier([]Rule{nodeModulesRule}, fakeFS{`d:\p`: {"package.json"}}).Classify(tr, testNow)
	if got := cls.Get(tr, find(t, tr, `D:\p\node_modules`)); got.Inactive {
		t.Error("recent sibling activity: want active")
	}
	// No other content → not inactive; non build-artifact never inactive.
	tr = buildTree(t, `D:\`, dirSpec{rel: `p`}, dirSpec{rel: `p\node_modules`}, dirSpec{rel: `q`, latest: now - 400*day}, dirSpec{rel: `q\cache`})
	cache := Rule{ID: "c", Category: "dev-cache", Match: Match{Dirname: "cache"}, Verdict: Safe, Action: ActionDeleteDir}
	cls = newTestClassifier([]Rule{nodeModulesRule, cache}, fakeFS{`d:\p`: {"package.json"}}).Classify(tr, testNow)
	if got := cls.Get(tr, find(t, tr, `D:\p\node_modules`)); got.Inactive || got.RuleID == "" {
		t.Errorf("empty parent: %+v", got)
	}
	if got := cls.Get(tr, find(t, tr, `D:\q\cache`)); got.Inactive || got.RuleID != "c" {
		t.Errorf("dev-cache: %+v", got)
	}
}

func TestCandidates(t *testing.T) {
	const mb = int64(1 << 20)
	tr := buildTree(t, `D:\`,
		dirSpec{rel: `big`, size: 600 * mb},
		dirSpec{rel: `small`, size: 400 * mb},
		// node_modules sits at the root so no unknown ancestor (other than the
		// skipped root) aggregates its 700 MB.
		dirSpec{rel: `node_modules`},
		dirSpec{rel: `node_modules\huge`, size: 700 * mb},
		dirSpec{rel: `link`, size: 900 * mb, flags: tree.FlagLink})
	cls := newTestClassifier([]Rule{nodeModulesRule}, fakeFS{`d:\`: {"package.json"}}).Classify(tr, testNow)
	got := Candidates(tr, cls, 500*mb)
	if len(got) != 1 || got[0] != find(t, tr, `D:\big`) {
		names := []string{}
		for _, id := range got {
			names = append(names, tr.Path(id))
		}
		t.Fatalf("candidates = %v", names)
	}
}

func TestCandidatesSortedAndUnderExact(t *testing.T) {
	const mb = int64(1 << 20)
	tr := buildTree(t, `C:\`, dirSpec{rel: `a`, size: 600 * mb}, dirSpec{rel: `Users`}, dirSpec{rel: `Users\dev`},
		dirSpec{rel: `Users\dev\Downloads`}, dirSpec{rel: `Users\dev\Downloads\isos`, size: 800 * mb},
		dirSpec{rel: `Windows`}, dirSpec{rel: `Windows\big`, size: 900 * mb})
	cls := newTestClassifier(nil, fakeFS{}).Classify(tr, testNow)
	got := Candidates(tr, cls, 500*mb)
	var paths []string
	for _, id := range got {
		paths = append(paths, tr.Path(id))
	}
	// Exact-protected Users\dev and Downloads are Keep themselves, but do not hide
	// their unknown content; C:\Windows\big is inside a protected subtree.
	// Ties keep tree order (Users before isos).
	want := []string{`C:\Users`, `C:\Users\dev\Downloads\isos`, `C:\a`}
	if strings.Join(paths, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", paths, want)
	}
}

func TestClassifyFileGlob(t *testing.T) {
	r := Rule{ID: "dumps", Category: "temp", Match: Match{Glob: "*.DMP", OlderThanDays: 7}, Verdict: Safe, Action: ActionDeleteDir}
	c := newTestClassifier([]Rule{nodeModulesRule, r}, fakeFS{})
	got, ok := c.ClassifyFile(`D:\crash\App.dmp`, testNow.Add(-8*24*time.Hour), testNow)
	if !ok || got.RuleID != "dumps" || got.Source != SourceRule || got.Verdict != Safe {
		t.Fatalf("8 days: %+v %v", got, ok)
	}
	if got, ok := c.ClassifyFile(`D:\crash\App.dmp`, testNow.Add(-6*24*time.Hour), testNow); ok {
		t.Fatalf("6 days: %+v", got)
	}
	if got, ok := c.ClassifyFile(`D:\crash\App.txt`, testNow.Add(-30*24*time.Hour), testNow); ok {
		t.Fatalf("txt: %+v", got)
	}
	got, ok = c.ClassifyFile(`C:\Windows\Minidump\x.dmp`, testNow.Add(-30*24*time.Hour), testNow)
	if !ok || got.Verdict != Keep || got.Source != SourceProtected {
		t.Fatalf("protected file: %+v %v", got, ok)
	}
	got, ok = c.ClassifyFile(`C:\pagefile.sys`, testNow, testNow)
	if !ok || got.Verdict != Keep || got.Source != SourceProtected {
		t.Fatalf("pagefile: %+v %v", got, ok)
	}
}

func TestRuleLookup(t *testing.T) {
	c := newTestClassifier([]Rule{nodeModulesRule}, fakeFS{})
	if r, ok := c.Rule("node.node_modules"); !ok || r.ID != "node.node_modules" {
		t.Fatal("lookup failed")
	}
	if _, ok := c.Rule("nope"); ok {
		t.Fatal("unexpected rule")
	}
}

func TestPathRuleEmptyEnvSkipped(t *testing.T) {
	r := Rule{ID: "e", Category: "temp", Match: Match{Path: `%EMPTY%\Users`}, Verdict: Safe, Action: ActionDeleteDir}
	tr := buildTree(t, `C:\`, dirSpec{rel: `Users`})
	cls := newTestClassifier([]Rule{r}, fakeFS{}).Classify(tr, testNow)
	if got := cls.Get(tr, find(t, tr, `C:\Users`)); got.Source != SourceUnknown {
		t.Fatalf("%+v", got)
	}
}

func TestWindowsTempClassified(t *testing.T) {
	sysTemp := Rule{ID: "windows.system_temp", Category: "temp", Match: Match{Path: `%SystemRoot%\Temp`}, Verdict: Safe, Action: ActionDeleteContents}
	tr := buildTree(t, `C:\`, dirSpec{rel: `Windows`}, dirSpec{rel: `Windows\System32`},
		dirSpec{rel: `Windows\Temp`}, dirSpec{rel: `Windows\Temp\x`}, dirSpec{rel: `Windows\Temp\x\.git`},
		dirSpec{rel: `Windows\Temp2`})

	// With the rule: Temp is the rule's, x inherits it.
	cls := newTestClassifier([]Rule{sysTemp}, fakeFS{}).Classify(tr, testNow)
	for _, p := range []string{`C:\Windows\Temp`, `C:\Windows\Temp\x`} {
		if got := cls.Get(tr, find(t, tr, p)); got.RuleID != "windows.system_temp" || got.Source != SourceRule || got.Verdict != Safe {
			t.Errorf("with rule %s: %+v", p, got)
		}
	}
	// Protection still applies around the exception. (A .git inside a consumed
	// rule match inherits the rule by design: HasProtectedDescendant excludes
	// .git and the cleaner handles it; see the no-rule case below.)
	for _, p := range []string{`C:\Windows\Temp2`, `C:\Windows\System32`} {
		if got := cls.Get(tr, find(t, tr, p)); got.Source != SourceProtected || got.Verdict != Keep {
			t.Errorf("with rule %s: %+v", p, got)
		}
	}

	// Without the rule: Temp and its content are unknown, not inherited Keep.
	cls = newTestClassifier(nil, fakeFS{}).Classify(tr, testNow)
	for _, p := range []string{`C:\Windows\Temp`, `C:\Windows\Temp\x`} {
		if got := cls.Get(tr, find(t, tr, p)); got.Source != SourceUnknown {
			t.Errorf("without rule %s: %+v", p, got)
		}
	}
	if got := cls.Get(tr, find(t, tr, `C:\Windows\Temp\x\.git`)); got.Source != SourceProtected {
		t.Errorf("without rule .git: %+v", got)
	}
	// A dirname rule applies inside the exception too.
	tmpX := Rule{ID: "x", Category: "temp", Match: Match{Dirname: "x"}, Verdict: Safe, Action: ActionDeleteDir}
	cls = newTestClassifier([]Rule{tmpX}, fakeFS{}).Classify(tr, testNow)
	if got := cls.Get(tr, find(t, tr, `C:\Windows\Temp\x`)); got.RuleID != "x" || got.Source != SourceRule {
		t.Errorf("dirname in Temp: %+v", got)
	}
	// Agreement with Protector.Check.
	p := newTestProtector()
	for _, s := range []string{`C:\Windows\Temp`, `C:\Windows\Temp\x`} {
		if p.Check(s) != ProtectNone {
			t.Errorf("Check(%s) != none", s)
		}
	}
}

func TestExactAncestorStopsInheritance(t *testing.T) {
	users := Rule{ID: "user.users", Category: "temp", Match: Match{Path: `C:\Users`}, Verdict: Safe, Action: ActionDeleteDir}
	tr := buildTree(t, `C:\`, dirSpec{rel: `Users`}, dirSpec{rel: `Users\dev`}, dirSpec{rel: `Users\dev\Documents`},
		dirSpec{rel: `Users\dev\Documents\thesis`}, dirSpec{rel: `Users\dev\AppData`}, dirSpec{rel: `Users\other`})
	cls := newTestClassifier([]Rule{users}, fakeFS{}).Classify(tr, testNow)
	if got := cls.Get(tr, find(t, tr, `C:\Users`)); got.RuleID != "user.users" {
		t.Fatalf("users: %+v", got)
	}
	if got := cls.Get(tr, find(t, tr, `C:\Users\other`)); got.RuleID != "user.users" {
		t.Fatalf("other: %+v", got)
	}
	for _, p := range []string{`C:\Users\dev\Documents\thesis`} {
		if got := cls.Get(tr, find(t, tr, p)); got.Source != SourceUnknown || got.Verdict != "" {
			t.Errorf("%s: %+v (must not inherit safe across an exact-protected ancestor)", p, got)
		}
	}
	for _, p := range []string{`C:\Users\dev`, `C:\Users\dev\Documents`, `C:\Users\dev\AppData`} {
		if got := cls.Get(tr, find(t, tr, p)); got.Source != SourceProtected {
			t.Errorf("%s: %+v", p, got)
		}
	}
}
