package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func validRule() Rule {
	return Rule{
		ID: "r1", Name: "R1", Category: "dev-cache",
		Match:   Match{Dirname: "node_modules"},
		Verdict: Safe, Action: ActionDeleteDir,
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		mut     func(*Rule)
		wantErr bool
	}{
		{"valid", func(r *Rule) {}, false},
		{"no id", func(r *Rule) { r.ID = "" }, true},
		{"no matcher", func(r *Rule) { r.Match = Match{} }, true},
		{"two matchers", func(r *Rule) { r.Match = Match{Dirname: "a", Path: "b"} }, true},
		{"bad verdict", func(r *Rule) { r.Verdict = "nope" }, true},
		{"bad category", func(r *Rule) { r.Category = "x" }, true},
		{"bad action", func(r *Rule) { r.Action = "x" }, true},
		{"manual no command", func(r *Rule) { r.Action = ActionManual }, true},
		{"manual with command", func(r *Rule) { r.Action = ActionManual; r.Command = "go clean" }, false},
		{"sibling without dirname", func(r *Rule) { r.Match = Match{Glob: "*.tmp", Sibling: "a"} }, true},
		{"sibling with dirname", func(r *Rule) { r.Match.Sibling = "*.csproj" }, false},
		{"older_than without glob", func(r *Rule) { r.Match.OlderThanDays = 3 }, true},
		{"older_than with glob", func(r *Rule) { r.Match = Match{Glob: "*.tmp", OlderThanDays: 3} }, false},
		{"negative older_than", func(r *Rule) { r.Match = Match{Glob: "*.tmp", OlderThanDays: -1} }, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := validRule()
			c.mut(&r)
			if err := Validate(r); (err != nil) != c.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
			}
		})
	}
}

const okYAML = `- id: ok1
  name: OK
  category: dev-cache
  match: {dirname: node_modules}
  verdict: safe
  action: delete_dir
`

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadUserDirSkipsInvalid(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "ok.yaml", okYAML)
	write(t, dir, "bad.yaml", "- id: bad1\n  category: dev-cache\n  match: {}\n  verdict: safe\n  action: delete_dir\n")
	write(t, dir, "broken.yaml", "- id: [unclosed\n  : :\n")
	write(t, dir, "ignored.txt", "whatever")
	rs, probs := LoadUserDir(dir)
	if len(rs) != 1 || rs[0].ID != "ok1" || rs[0].Builtin {
		t.Fatalf("rules=%+v", rs)
	}
	if len(probs) != 2 {
		t.Fatalf("problems=%+v", probs)
	}
	for _, p := range probs {
		if p.File == "" || p.Err == "" {
			t.Fatalf("incomplete problem %+v", p)
		}
	}
}

func TestLoadUserDirUnknownKey(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "typo.yaml", "- id: t1\n  categroy: dev-cache\n")
	rs, probs := LoadUserDir(dir)
	if len(rs) != 0 || len(probs) != 1 {
		t.Fatalf("rules=%v probs=%v", rs, probs)
	}
}

func TestLoadUserDirMissing(t *testing.T) {
	rs, probs := LoadUserDir(filepath.Join(t.TempDir(), "nope"))
	if len(rs) != 0 || len(probs) != 0 {
		t.Fatal("expected nothing")
	}
}

func TestLoadUserDirDuplicateID(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.yaml", okYAML+"- id: ok1\n  category: temp\n  match: {dirname: other}\n  verdict: caution\n  action: delete_dir\n")
	rs, probs := LoadUserDir(dir)
	if len(rs) != 1 || rs[0].Category != "dev-cache" || len(probs) != 1 {
		t.Fatalf("rules=%+v probs=%+v", rs, probs)
	}
}

func TestLoadBuiltin(t *testing.T) {
	rs, err := LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rs {
		if !r.Builtin {
			t.Fatalf("not marked builtin: %s", r.ID)
		}
	}
}

func TestMostConservative(t *testing.T) {
	cases := []struct{ a, b, want Verdict }{
		{Safe, Caution, Caution}, {Keep, Safe, Keep}, {Caution, Keep, Keep}, {Safe, Safe, Safe},
	}
	for _, c := range cases {
		if got := MostConservative(c.a, c.b); got != c.want {
			t.Errorf("(%s,%s)=%s want %s", c.a, c.b, got, c.want)
		}
	}
}
