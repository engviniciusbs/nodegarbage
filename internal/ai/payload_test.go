package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
	"github.com/engviniciusbs/nodegarbage/internal/tree"
	"github.com/engviniciusbs/nodegarbage/internal/volumes"
)

const mb = int64(1024 * 1024)

func TestAnonymizeUser(t *testing.T) {
	a := NewAnonymizer("Vinicius", nil)
	cases := map[string]string{
		`C:\Users\Vinicius\src`:   `C:\Users\<user>\src`,
		`c:\users\VINICIUS\x`:     `c:\users\<user>\x`,
		`C:\Users\Public`:         `C:\Users\Public`,
		`D:\backup\vinicius\docs`: `D:\backup\<user>\docs`,
	}
	for in, want := range cases {
		if got := a.Apply(in); got != want {
			t.Errorf("Apply(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAnonymizeExtra(t *testing.T) {
	a := NewAnonymizer("Vinicius", []string{"acme", ""})
	if got, want := a.Apply(`D:\clientes\ACME\app`), `D:\clientes\<redacted>\app`; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if got, want := a.Apply(`D:\x\myacmecorp`), `D:\x\my<redacted>corp`; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

var rule = rules.Rule{
	ID: "node-modules", Category: "dev-cache", Verdict: rules.Safe, Action: rules.ActionDeleteDir,
	Match: rules.Match{Dirname: "node_modules"},
}

func classify(tr *tree.Tree) *rules.Classifications {
	env := func(string) string { return "" }
	p := rules.NewProtector(env, []string{`C:\`}, nil)
	c := rules.NewClassifier([]rules.Rule{rule}, p, env,
		func(string) bool { return false }, func(string) []string { return nil })
	return c.Classify(tr, time.Now())
}

type dir struct {
	rel  string
	size int64
}

func build(dirs ...dir) *tree.Tree {
	b := tree.NewBuilder(`C:\`, 100)
	ids := map[string]tree.NodeID{"": b.AddDir(tree.NoNode, `C:\`, 0)}
	for _, d := range dirs {
		parent, name := "", d.rel
		if i := strings.LastIndex(d.rel, `\`); i >= 0 {
			parent, name = d.rel[:i], d.rel[i+1:]
		}
		id := b.AddDir(ids[parent], name, 0)
		b.SetOwnStats(id, d.size, 1, 1000)
		ids[d.rel] = id
	}
	return b.Finish()
}

func scan(tr *tree.Tree) []ScanData {
	return []ScanData{{Volume: volumes.Volume{Root: `C:\`, FS: "NTFS", Total: 1000, Free: 400}, Tree: tr, Classes: classify(tr)}}
}

func TestPayloadLimits(t *testing.T) {
	var dirs []dir
	for i := 0; i < 300; i++ {
		p := fmt.Sprintf("p%03d", i)
		dirs = append(dirs, dir{p, 0}, dir{p + `\node_modules`, int64(1000 + i)})
	}
	for i := 0; i < 200; i++ {
		dirs = append(dirs, dir{fmt.Sprintf("u%03d", i), int64(5000 + i)})
	}
	p, _ := BuildPayload(scan(build(dirs...)), NewAnonymizer("x", nil), PayloadOptions{BudgetTokens: 1 << 30, MinUnknownBytes: 1})
	if len(p.Classified) != 200 || len(p.Unknown) != 150 {
		t.Fatalf("classified=%d unknown=%d", len(p.Classified), len(p.Unknown))
	}
	for i := 1; i < len(p.Classified); i++ {
		if p.Classified[i].Size > p.Classified[i-1].Size {
			t.Fatal("classified not desc")
		}
	}
	for i := 1; i < len(p.Unknown); i++ {
		if p.Unknown[i].Size > p.Unknown[i-1].Size {
			t.Fatal("unknown not desc")
		}
	}
	if p.Classified[0].RuleID != "node-modules" || p.Classified[0].Verdict != rules.Safe {
		t.Errorf("bad classified item %+v", p.Classified[0])
	}
}

func TestUnknownSizeExcludesClassified(t *testing.T) {
	tr := build(dir{"proj", 500 * mb}, dir{`proj\node_modules`, 700 * mb})
	p, _ := BuildPayload(scan(tr), NewAnonymizer("x", nil), PayloadOptions{MinUnknownBytes: 400 * mb})
	if len(p.Unknown) != 1 || p.Unknown[0].Size != 500*mb {
		t.Fatalf("unknown = %+v", p.Unknown)
	}
	if len(p.Classified) != 1 || p.Classified[0].Size != 700*mb {
		t.Fatalf("classified = %+v", p.Classified)
	}
	p, _ = BuildPayload(scan(tr), NewAnonymizer("x", nil), PayloadOptions{MinUnknownBytes: 600 * mb})
	if len(p.Unknown) != 0 {
		t.Fatalf("expected proj excluded at 600MB, got %+v", p.Unknown)
	}
}

func TestDefaultsApply(t *testing.T) {
	p, _ := BuildPayload(scan(build(dir{"small", 10 * mb}, dir{"big", 600 * mb})), NewAnonymizer("x", nil), PayloadOptions{})
	if len(p.Unknown) != 1 || !strings.HasSuffix(p.Unknown[0].Path, `big`) {
		t.Fatalf("unknown = %+v", p.Unknown)
	}
}

func manyDirs() []dir {
	var dirs []dir
	for i := 0; i < 300; i++ {
		p := fmt.Sprintf("projeto-com-nome-longo-%03d", i)
		dirs = append(dirs, dir{p, 0}, dir{p + `\node_modules`, int64(1000 + i)})
	}
	for i := 0; i < 200; i++ {
		dirs = append(dirs, dir{fmt.Sprintf("pasta-desconhecida-%03d", i), int64(5000 + i)})
	}
	return dirs
}

func TestBudgetTrimsSmallestFirst(t *testing.T) {
	tr := build(manyDirs()...)
	scans := scan(tr)
	full, _ := BuildPayload(scans, NewAnonymizer("x", nil), PayloadOptions{BudgetTokens: 1 << 30, MinUnknownBytes: 1})
	p, _ := BuildPayload(scans, NewAnonymizer("x", nil), PayloadOptions{BudgetTokens: 2000, MinUnknownBytes: 1})
	b, err := p.Compact()
	if err != nil {
		t.Fatal(err)
	}
	if n := EstimateTokens(b); n > 2000 {
		t.Fatalf("tokens = %d", n)
	}
	if len(p.Classified) == 0 || p.Classified[0].Path != full.Classified[0].Path {
		t.Error("largest classified item lost")
	}
	if len(p.Unknown) == 0 || p.Unknown[0].Path != full.Unknown[0].Path {
		t.Error("largest unknown item lost")
	}
	if len(p.Classified) >= len(full.Classified) || len(p.Unknown) >= len(full.Unknown) {
		t.Error("expected both lists trimmed")
	}
	// smallest dropped: what remains is a prefix of the full (desc) list
	for i, it := range p.Unknown {
		if it.Path != full.Unknown[i].Path {
			t.Fatalf("unknown[%d] not a prefix of the full list", i)
		}
	}
}

func TestPathMapRoundTrip(t *testing.T) {
	tr := build(manyDirs()...)
	p, pm := BuildPayload(scan(tr), NewAnonymizer("x", nil), PayloadOptions{BudgetTokens: 2000, MinUnknownBytes: 1})
	check := func(anon string) {
		real, ok := pm[rules.NormalizePath(anon)]
		if !ok {
			t.Fatalf("%q missing from PathMap", anon)
		}
		if _, found := tr.Find(real); !found {
			t.Fatalf("%q does not map to a real path: %q", anon, real)
		}
	}
	for _, it := range p.Classified {
		check(it.Path)
	}
	for _, it := range p.Unknown {
		check(it.Path)
	}
}

func TestNoRealUserNameInPayload(t *testing.T) {
	tr := build(
		dir{"Users", 0}, dir{`Users\Vinicius`, 0},
		dir{`Users\Vinicius\big`, 600 * mb}, dir{`Users\Vinicius\big\node_modules`, 10 * mb},
		dir{`Users\Vinicius\acme-stuff`, 600 * mb},
	)
	tr.TopFiles = []tree.FileInfo{{Path: `C:\Users\Vinicius\big\a.bin`, Size: 5, ModTime: 1}, {Path: `C:\users\VINICIUS\big\b.iso`, Size: 4, ModTime: 1}}
	p, pm := BuildPayload(scan(tr), NewAnonymizer("Vinicius", []string{"acme"}), PayloadOptions{})
	out, err := p.Compact()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(out)), "vinicius") || strings.Contains(strings.ToLower(string(out)), "acme") {
		t.Fatalf("leak: %s", out)
	}
	if !strings.Contains(string(out), `C:\\Users\\<user>\\big`) {
		t.Errorf("expected anonymized path in %s", out)
	}
	if len(p.TopFiles) != 2 || pm[rules.NormalizePath(p.TopFiles[0].Path)] != `C:\Users\Vinicius\big\a.bin` {
		t.Errorf("topfiles/pathmap wrong: %+v", p.TopFiles)
	}
}

func TestCollisionKeepsFirst(t *testing.T) {
	tr := build(dir{"acme1", 700 * mb}, dir{"acme2", 600 * mb})
	p, pm := BuildPayload(scan(tr), NewAnonymizer("x", []string{"acme1", "acme2"}), PayloadOptions{})
	if len(p.Unknown) != 1 {
		t.Fatalf("unknown = %+v", p.Unknown)
	}
	if got := pm[rules.NormalizePath(p.Unknown[0].Path)]; got != `C:\acme1` {
		t.Errorf("pathmap value = %q", got)
	}
}

func TestCompactShapeDeterministic(t *testing.T) {
	tr := build(dir{"big", 600 * mb})
	p, _ := BuildPayload(scan(tr), NewAnonymizer("x", nil), PayloadOptions{})
	a, _ := p.Compact()
	b, _ := p.Compact()
	if string(a) != string(b) {
		t.Fatal("not deterministic")
	}
	var m map[string]struct {
		Cols []string `json:"cols"`
		Rows [][]any  `json:"rows"`
	}
	if err := json.Unmarshal(a, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"volumes", "classified", "unknown", "extensions", "topFiles"} {
		if len(m[k].Cols) == 0 || m[k].Rows == nil {
			t.Errorf("list %s malformed: %s", k, a)
		}
	}
}

func TestNoNameLeakInSubstringsAndExtensions(t *testing.T) {
	tr := build(
		dir{"dev", 0}, dir{`dev\vinicius-backup`, 600 * mb},
		dir{"vinicius.config", 700 * mb},
		dir{"x", 0}, dir{`x\vinicius_old`, 650 * mb},
		dir{"ok", 800 * mb},
	)
	tr.TopFiles = []tree.FileInfo{
		{Path: `C:\Users\Vinicius\Documents\Vinicius CV.pdf`, Size: 9, ModTime: 1},
		{Path: `C:\ok\a.vinicius`, Size: 8, ModTime: 1},
		{Path: `C:\ok\b.iso`, Size: 7, ModTime: 1},
	}
	tr.Extensions = []tree.ExtStat{{Ext: ".vinicius", Count: 1, Bytes: 8}, {Ext: ".iso", Count: 1, Bytes: 7}}
	p, pm := BuildPayload(scan(tr), NewAnonymizer("Vinicius", nil), PayloadOptions{})
	out, err := p.Compact()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(out)), "vinicius") {
		t.Fatalf("leak: %s", out)
	}
	if len(p.Extensions) != 2 || p.Extensions[0].Ext != ".<user>" {
		t.Errorf("extensions = %+v", p.Extensions)
	}
	if len(p.Unknown) != 6 {
		t.Fatalf("unknown = %+v", p.Unknown)
	}
	for _, it := range p.Unknown {
		if strings.HasSuffix(it.Path, "ok") && len(it.Exts) != 2 {
			t.Errorf("ok exts = %v", it.Exts)
		}
	}
	for k, v := range pm {
		if strings.Contains(k, "vinicius") && !strings.Contains(k, "<user>") {
			t.Errorf("bad key %q -> %q", k, v)
		}
	}
	if len(pm) != len(p.Unknown)+len(p.Classified)+len(p.TopFiles) {
		t.Errorf("pathmap inconsistent: %d entries", len(pm))
	}
}

func TestLeakGuardDropsRows(t *testing.T) {
	// Substring replacement handles 4+ char names; Leaks is the defense-in-depth guard.
	if g := NewAnonymizer("Vinicius", nil); !g.Leaks(`xVINICIUS.txt`) || g.Leaks(`x<user>.txt`) {
		t.Error("Leaks misbehaves")
	}
	tr := build(dir{"my-zed1-stuff", 600 * mb})
	p, _ := BuildPayload(scan(tr), NewAnonymizer("zed1", nil), PayloadOptions{})
	out, _ := p.Compact()
	if strings.Contains(strings.ToLower(string(out)), "zed1") {
		t.Fatalf("leak: %s", out)
	}
	if len(p.Unknown) != 1 || !strings.Contains(p.Unknown[0].Path, "<user>") {
		t.Errorf("unknown = %+v", p.Unknown)
	}
}

func TestShortNameWholeComponentOnly(t *testing.T) {
	a := NewAnonymizer("dev", nil)
	if got := a.Apply(`C:\devtools\x`); got != `C:\devtools\x` {
		t.Errorf("mangled: %q", got)
	}
	if got := a.Apply(`C:\Users\DEV\x`); got != `C:\Users\<user>\x` {
		t.Errorf("got %q", got)
	}
	if a.Leaks(`C:\devtools`) {
		t.Error("guard must be off for short names")
	}
	b := NewAnonymizer("Vinicius", nil)
	if got := b.Apply(`C:\x\vinicius-backup\Vinicius CV.pdf`); got != `C:\x\<user>-backup\<user> CV.pdf` {
		t.Errorf("got %q", got)
	}
}

// Classified lists rule-classified items only (spec §5): a protected node
// without a rule (e.g. $Recycle.Bin) is neither classified nor unknown.
func TestClassifiedExcludesPlainProtected(t *testing.T) {
	tr := build(dir{"$Recycle.Bin", 700 * mb}, dir{"p", 0}, dir{`p\node_modules`, 10 * mb})
	p, _ := BuildPayload(scan(tr), NewAnonymizer("x", nil), PayloadOptions{MinUnknownBytes: 1})
	for _, it := range p.Classified {
		if it.RuleID == "" || strings.Contains(it.Path, "Recycle") {
			t.Errorf("plain protected node in classified: %+v", it)
		}
	}
	for _, it := range p.Unknown {
		if strings.Contains(it.Path, "Recycle") {
			t.Errorf("protected node in unknown: %+v", it)
		}
	}
	if len(p.Classified) != 1 || p.Classified[0].RuleID != "node-modules" {
		t.Errorf("classified = %+v", p.Classified)
	}
}
