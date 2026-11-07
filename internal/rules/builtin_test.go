package rules

import (
	"strings"
	"testing"
	"time"
)

// builtinEnv extends fakeEnv with %APPDATA%.
func builtinEnv(key string) string {
	if strings.EqualFold(key, "APPDATA") {
		return `C:\Users\dev\AppData\Roaming`
	}
	return fakeEnv(key)
}

func loadBuiltinClassifier(t *testing.T, fs fakeFS) (*Classifier, []Rule) {
	t.Helper()
	rs, err := LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	p := NewProtector(builtinEnv, []string{`C:\`, `D:\`}, nil)
	return NewClassifier(rs, p, builtinEnv, fs.exists, fs.listNames), rs
}

// treeForPath returns dirSpecs for every directory on the way to path
// (relative to its drive root).
func treeForPath(t *testing.T, path string) []dirSpec {
	t.Helper()
	parts := strings.Split(path, `\`)
	var specs []dirSpec
	for i := 1; i < len(parts); i++ {
		specs = append(specs, dirSpec{rel: strings.Join(parts[1:i+1], `\`)})
	}
	return specs
}

func TestBuiltinLoads(t *testing.T) {
	rs, err := LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) < 30 {
		t.Fatalf("only %d built-in rules", len(rs))
	}
	seen := map[string]bool{}
	for _, r := range rs {
		if seen[r.ID] {
			t.Errorf("duplicate id %s", r.ID)
		}
		seen[r.ID] = true
		if r.Name == "" || strings.TrimSpace(r.Explain) == "" {
			t.Errorf("%s: name and explain are required", r.ID)
		}
		if !r.Builtin {
			t.Errorf("%s: Builtin flag not set", r.ID)
		}
	}
}

func TestBuiltinTable(t *testing.T) {
	const (
		u  = `C:\Users\dev`
		la = `C:\Users\dev\AppData\Local`
	)
	cases := []struct {
		id       string
		path     string
		siblings []string // names in the parent dir (for dirname+sibling rules)
		verdict  Verdict
		action   Action
		command  string // expected command for manual rules
	}{
		{"node.node_modules", `D:\proj\node_modules`, []string{"package.json", "node_modules"}, Safe, ActionDeleteDir, ""},
		{"node.npm_cache", la + `\npm-cache`, nil, Safe, ActionDeleteDir, ""},
		{"node.pnpm_store", la + `\pnpm\store`, nil, Safe, ActionManual, "pnpm store prune"},
		{"node.yarn_cache", la + `\Yarn\Cache`, nil, Safe, ActionDeleteDir, ""},
		{"node.bun_cache", u + `\.bun\install\cache`, nil, Safe, ActionDeleteDir, ""},
		{"go.build_cache", la + `\go-build`, nil, Safe, ActionManual, "go clean -cache"},
		{"go.mod_cache", u + `\go\pkg\mod`, nil, Safe, ActionManual, "go clean -modcache"},
		{"rust.target", `D:\crate\target`, []string{"Cargo.toml", "target"}, Safe, ActionDeleteDir, ""},
		{"rust.registry", u + `\.cargo\registry`, nil, Safe, ActionDeleteDir, ""},
		{"dotnet.bin", `D:\app\bin`, []string{"App.csproj", "bin"}, Safe, ActionDeleteDir, ""},
		{"dotnet.obj", `D:\app\obj`, []string{"App.csproj", "obj"}, Safe, ActionDeleteDir, ""},
		{"dotnet.bin_sln", `D:\sol\bin`, []string{"App.sln", "bin"}, Safe, ActionDeleteDir, ""},
		{"dotnet.obj_sln", `D:\sol\obj`, []string{"App.sln", "obj"}, Safe, ActionDeleteDir, ""},
		{"dotnet.nuget", u + `\.nuget\packages`, nil, Safe, ActionManual, "dotnet nuget locals all --clear"},
		{"python.pycache", `D:\py\pkg\__pycache__`, nil, Safe, ActionDeleteDir, ""},
		{"python.venv", `D:\py\.venv`, []string{"pyproject.toml", ".venv"}, Caution, ActionDeleteDir, ""},
		{"python.venv_requirements", `D:\rq\.venv`, []string{"requirements.txt", ".venv"}, Caution, ActionDeleteDir, ""},
		{"python.pip_cache", la + `\pip\Cache`, nil, Safe, ActionDeleteDir, ""},
		{"python.uv_cache", la + `\uv\cache`, nil, Safe, ActionManual, "uv cache clean"},
		{"jvm.gradle_caches", u + `\.gradle\caches`, nil, Safe, ActionDeleteDir, ""},
		{"jvm.gradle_build", `D:\jv\build`, []string{"build.gradle.kts", "build"}, Safe, ActionDeleteDir, ""},
		{"jvm.maven_repo", u + `\.m2\repository`, nil, Caution, ActionDeleteDir, ""},
		{"android.avd", u + `\.android\avd`, nil, Caution, ActionManual, "Excluir AVDs pelo Android Studio (Device Manager)"},
		{"containers.docker_vhdx", la + `\Docker\wsl`, nil, Caution, ActionManual, "docker system prune -a"},
		{"windows.user_temp", la + `\Temp`, nil, Safe, ActionDeleteContents, ""},
		{"windows.system_temp", `C:\Windows\Temp`, nil, Safe, ActionDeleteContents, ""},
		{"windows.crashdumps", la + `\CrashDumps`, nil, Safe, ActionDeleteContents, ""},
		{"browsers.chrome_cache", la + `\Google\Chrome\User Data\Default\Cache`, nil, Safe, ActionDeleteContents, ""},
		{"browsers.edge_cache", la + `\Microsoft\Edge\User Data\Default\Cache`, nil, Safe, ActionDeleteContents, ""},
		{"browsers.firefox_cache", la + `\Mozilla\Firefox\Profiles`, nil, Caution, ActionManual, "about:preferences#privacy → Limpar dados"},
		{"editors.vscode_cache", u + `\AppData\Roaming\Code\Cache`, nil, Safe, ActionDeleteContents, ""},
		{"editors.vscode_cacheddata", u + `\AppData\Roaming\Code\CachedData`, nil, Safe, ActionDeleteContents, ""},
		{"editors.jetbrains_cache", la + `\JetBrains`, nil, Caution, ActionManual,
			"Use 'Invalidate Caches' na IDE ou apague apenas as pastas de cache de versões antigas"},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			fs := fakeFS{}
			parent := c.path[:strings.LastIndex(c.path, `\`)]
			if c.siblings != nil {
				fs[NormalizePath(parent)] = c.siblings
			}
			cl, _ := loadBuiltinClassifier(t, fs)
			tr := buildTree(t, c.path[:3], treeForPath(t, c.path)...)
			got := cl.Classify(tr, testNow).Get(tr, find(t, tr, c.path))
			if got.RuleID != c.id || got.Source != SourceRule || got.Verdict != c.verdict || got.Action != c.action {
				t.Fatalf("got %+v", got)
			}
			r, ok := cl.Rule(c.id)
			if !ok {
				t.Fatal("rule lookup failed")
			}
			if r.Action == ActionManual && r.Command != c.command {
				t.Fatalf("command = %q, want %q", r.Command, c.command)
			}
		})
	}
	// Every built-in rule has a table case (the glob rule is covered separately).
	rs, _ := LoadBuiltin()
	covered := map[string]bool{"containers.wsl_vhdx": true, "windows.update_download": true}
	for _, c := range cases {
		covered[c.id] = true
	}
	for _, r := range rs {
		if !covered[r.ID] {
			t.Errorf("rule %s has no table case", r.ID)
		}
	}
}

func TestBuiltinWSLVhdx(t *testing.T) {
	cl, _ := loadBuiltinClassifier(t, fakeFS{})
	got, ok := cl.ClassifyFile(`C:\Users\dev\AppData\Local\Packages\X\LocalState\ext4.vhdx`, testNow, testNow)
	if !ok || got.RuleID != "containers.wsl_vhdx" || got.Verdict != Caution || got.Action != ActionManual {
		t.Fatalf("%+v %v", got, ok)
	}
	if r, _ := cl.Rule("containers.wsl_vhdx"); r.Command != "wsl --manage <distro> --set-sparse true" {
		t.Fatalf("command %q", r.Command)
	}
	if _, ok := cl.ClassifyFile(`D:\x\disk.vhdx`, time.Time{}, testNow); ok {
		t.Fatal("other vhdx must not match")
	}
}

func TestBuiltinSiblingRequired(t *testing.T) {
	cl, _ := loadBuiltinClassifier(t, fakeFS{`d:\a`: {"readme.md"}})
	tr := buildTree(t, `D:\`, dirSpec{rel: `a`}, dirSpec{rel: `a\node_modules`}, dirSpec{rel: `a\target`},
		dirSpec{rel: `a\bin`}, dirSpec{rel: `a\.venv`}, dirSpec{rel: `a\build`})
	cls := cl.Classify(tr, testNow)
	for _, n := range []string{"node_modules", "target", "bin", ".venv", "build"} {
		if got := cls.Get(tr, find(t, tr, `D:\a\`+n)); got.Source != SourceUnknown {
			t.Errorf("%s without sibling: %+v", n, got)
		}
	}
}

func TestBuiltinUpdateDownloadProtected(t *testing.T) {
	cl, _ := loadBuiltinClassifier(t, fakeFS{})
	p := `C:\Windows\SoftwareDistribution\Download`
	tr := buildTree(t, `C:\`, treeForPath(t, p)...)
	got := cl.Classify(tr, testNow).Get(tr, find(t, tr, p))
	if got.RuleID != "windows.update_download" || got.Verdict != Keep || got.Source != SourceProtected {
		t.Fatalf("%+v", got)
	}
	r, _ := cl.Rule("windows.update_download")
	if r.Verdict != Caution || r.Action != ActionManual || r.Command != "Use Limpeza de Disco → Limpar arquivos do sistema" {
		t.Fatalf("rule %+v", r)
	}
}

func TestBuiltinNeverUnprotects(t *testing.T) {
	cl, rs := loadBuiltinClassifier(t, fakeFS{})
	prot := NewProtector(builtinEnv, []string{`C:\`, `D:\`}, nil)
	n := 0
	for _, r := range rs {
		if r.Match.Path == "" {
			continue
		}
		exp, ok := expandEnv(r.Match.Path, builtinEnv)
		if !ok {
			t.Errorf("%s: path %q does not expand", r.ID, r.Match.Path)
			continue
		}
		if prot.Check(exp) != ProtectSubtree {
			continue
		}
		n++
		tr := buildTree(t, exp[:3], treeForPath(t, exp)...)
		if got := cl.Classify(tr, testNow).Get(tr, find(t, tr, exp)); got.Verdict != Keep {
			t.Errorf("%s: protected path classified %+v", r.ID, got)
		}
	}
	if n == 0 {
		t.Fatal("expected at least one rule inside a protected subtree (windows.update_download)")
	}
}
