package rules

import (
	"strings"
	"testing"
)

var fakeVars = map[string]string{
	"SystemRoot":         `C:\Windows`,
	"ProgramFiles":       `C:\Program Files`,
	"ProgramFiles(x86)":  `C:\Program Files (x86)`,
	"ProgramData":        `C:\ProgramData`,
	"USERPROFILE":        `C:\Users\dev`,
	"LOCALAPPDATA":       `C:\Users\dev\AppData\Local`,
	"TEMP":               `C:\Users\dev\AppData\Local\Temp`,
	"EMPTY":              "",
	"UnusedButPresent_x": "x",
}

// fakeEnv mimics os.Getenv on Windows: names are case-insensitive.
func fakeEnv(key string) string {
	for k, v := range fakeVars {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

func newTestProtector(extra ...string) *Protector {
	return NewProtector(fakeEnv, []string{`C:\`, `D:\`}, extra)
}

func expectKind(t *testing.T, p *Protector, path string, want ProtectKind) {
	t.Helper()
	if got := p.Check(path); got != want {
		t.Errorf("Check(%q) = %v, want %v", path, got, want)
	}
}

func TestNormalizePath(t *testing.T) {
	cases := map[string]string{
		`C:\Windows\`:           `c:\windows`,
		`C:/Windows/System32/`:  `c:\windows\system32`,
		`\\?\C:\Windows`:        `c:\windows`,
		`C:`:                    `c:\`,
		`C:\`:                   `c:\`,
		`\\?\C:\`:               `c:\`,
		`C:\\Users\\dev`:        `c:\users\dev`,
		`C:\Users\.\dev\..\dev`: `c:\users\dev`,
		`C:\..\..\Windows`:      `c:\windows`,
		`\\?\UNC\srv\share\x`:   `\\srv\share\x`,
		`\\srv\share\`:          `\\srv\share`,
		`\\.\C:\Windows\x`:      `c:\windows\x`,
		`\\.\UNC\srv\share\x`:   `\\srv\share\x`,
		`C:\Windows.\System32`:  `c:\windows\system32`,
		`C:\Windows \x`:         `c:\windows\x`,
		`D:\a\.git.`:            `d:\a\.git`,
		`D:\a\...\b`:            `d:\a\b`,
	}
	for in, want := range cases {
		if got := NormalizePath(in); got != want {
			t.Errorf("NormalizePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProtectCaseAndForm(t *testing.T) {
	p := newTestProtector()
	for _, s := range []string{
		`c:\windows\SYSTEM32`, `C:/Windows/System32/`, `\\?\C:\Windows`, `C:\Windows`,
		`C:\Program Files\x`, `C:\Program Files (x86)`, `C:\ProgramData\Microsoft\Windows`,
		`C:\System Volume Information`, `D:\$Recycle.Bin\S-1-5`,
		`C:\Users\dev\..\..\Windows\System32`,
		`\\.\C:\Windows\x`, `C:\Windows.\System32`, `C:\Windows \x`, `C:\Windows. .\x`,
		`D:\src\.git.\objects`, `D:\src\.git `, `\\?\C:\Program Files.\x`,
	} {
		expectKind(t, p, s, ProtectSubtree)
	}
	// Component-aware prefix: siblings with a common string prefix are not protected.
	for _, s := range []string{`C:\Windows2`, `C:\Program Files Extra`, `C:\ProgramData\MicrosoftEdgeX`, `C:\ProgramData\Other`} {
		expectKind(t, p, s, ProtectNone)
	}
}

func TestWindowsTempException(t *testing.T) {
	p := newTestProtector()
	expectKind(t, p, `C:\Windows\Temp`, ProtectNone)
	expectKind(t, p, `C:\Windows\Temp\x`, ProtectNone)
	expectKind(t, p, `c:/windows/temp/`, ProtectNone)
	expectKind(t, p, `C:\Windows\Temp2`, ProtectSubtree)
}

func TestExactOnly(t *testing.T) {
	p := newTestProtector()
	for _, s := range []string{
		`C:\Users\dev\Documents`, `C:\Users\dev`, `c:\users\DEV\downloads\`, `C:\`, `D:`, `D:\pagefile.sys`,
		`C:\hiberfil.sys`, `C:\swapfile.sys`, `C:\Users\dev\Desktop`, `C:\Users\dev\Music`,
	} {
		expectKind(t, p, s, ProtectExact)
	}
	expectKind(t, p, `C:\Users\dev\Documents\proj\node_modules`, ProtectNone)
	expectKind(t, p, `C:\Users\dev\AppData`, ProtectNone)
	expectKind(t, p, `C:\pagefile.sys.bak`, ProtectNone)
	expectKind(t, p, `E:\`, ProtectNone) // not a known volume root
}

func TestGitSubtree(t *testing.T) {
	p := newTestProtector()
	expectKind(t, p, `D:\src\app\.git\objects`, ProtectSubtree)
	expectKind(t, p, `D:\src\app\.GIT`, ProtectSubtree)
	expectKind(t, p, `D:\src\app\.github`, ProtectNone)
	expectKind(t, p, `D:\src\app\x.git`, ProtectNone)
}

func TestHasProtectedDescendant(t *testing.T) {
	p := newTestProtector(`D:\vault`)
	cases := map[string]bool{
		`C:\Users\dev`:                false, // set below
		`C:\Users\dev\Documents\proj`: false,
		`C:\Users`:                    true,
		`C:\`:                         true,
		`D:\`:                         true,  // D:\vault, $Recycle.Bin, ...
		`D:\src\app`:                  false, // .git excluded
		`C:\Users\dev\AppData`:        false,
		`C:\Windows\System32`:         true, // inside a protected subtree
		`C:\Windows\Temp`:             false,
		`C:\Users\dev\Documents`:      false, // the path itself is not "below"
	}
	cases[`C:\Users\dev`] = true // Documents etc.
	for path, want := range cases {
		if got := p.HasProtectedDescendant(path); got != want {
			t.Errorf("HasProtectedDescendant(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestExtraPaths(t *testing.T) {
	p := newTestProtector(`D:\vault`, `%EMPTY%\x`, `%LOCALAPPDATA%\Keep`)
	expectKind(t, p, `D:\vault\a`, ProtectSubtree)
	expectKind(t, p, `d:\VAULT`, ProtectSubtree)
	expectKind(t, p, `D:\vault2`, ProtectNone)
	expectKind(t, p, `C:\Users\dev\AppData\Local\Keep\z`, ProtectSubtree)
	// An entry whose variable expands to "" never matches (would otherwise be `\x`).
	expectKind(t, p, `\x`, ProtectNone)
	// Extra paths are protected even inside the %SystemRoot%\Temp exception.
	p2 := newTestProtector(`C:\Windows\Temp\mine`)
	expectKind(t, p2, `C:\Windows\Temp\mine\a`, ProtectSubtree)
	expectKind(t, p2, `C:\Windows\Temp\other`, ProtectNone)
}

func TestMissingEnvSkipsEntry(t *testing.T) {
	env := func(k string) string {
		if strings.EqualFold(k, "SystemRoot") {
			return `C:\Windows`
		}
		return ""
	}
	p := NewProtector(env, []string{`C:\`}, nil)
	expectKind(t, p, `C:\Windows\x`, ProtectSubtree)
	expectKind(t, p, `C:\Program Files`, ProtectNone)
	expectKind(t, p, `C:\Documents`, ProtectNone) // %USERPROFILE%\Documents with empty USERPROFILE
	expectKind(t, p, `\Documents`, ProtectNone)
}

func TestExpandEnv(t *testing.T) {
	got, ok := expandEnv(`%programfiles(X86)%\a`, fakeEnv)
	if !ok || got != `C:\Program Files (x86)\a` {
		t.Fatalf("got %q %v", got, ok)
	}
	if _, ok := expandEnv(`%EMPTY%\a`, fakeEnv); ok {
		t.Fatal("empty var must fail")
	}
	if _, ok := expandEnv(`%NOPE%\a`, fakeEnv); ok {
		t.Fatal("missing var must fail")
	}
	if got, ok := expandEnv(`C:\100%`, fakeEnv); !ok || got != `C:\100%` {
		t.Fatalf("lone percent: %q %v", got, ok)
	}
}

func TestSetExtraAppliesImmediately(t *testing.T) {
	p := newTestProtector(`D:\old`)
	expectKind(t, p, `D:\old\x`, ProtectSubtree)
	expectKind(t, p, `D:\new\x`, ProtectNone)
	p.SetExtra([]string{`D:\new`, `%USERPROFILE%\keepme`, `%EMPTY%\skipped`, ""})
	expectKind(t, p, `D:\new\x`, ProtectSubtree)
	expectKind(t, p, `C:\Users\dev\keepme\a`, ProtectSubtree)
	expectKind(t, p, `D:\old\x`, ProtectNone) // replaced, not appended
	if !p.HasProtectedDescendant(`D:\`) {
		t.Fatal("HasProtectedDescendant must see new extras")
	}
	p.SetExtra(nil)
	expectKind(t, p, `D:\new\x`, ProtectNone)
	expectKind(t, p, `C:\Windows\x`, ProtectSubtree) // builtin untouched
}

func TestSetExtraConcurrentWithCheck(t *testing.T) {
	p := newTestProtector()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			p.SetExtra([]string{`D:\a`})
			p.SetExtra(nil)
		}
	}()
	for i := 0; i < 500; i++ {
		p.Check(`D:\a\b`)
		p.HasProtectedDescendant(`D:\`)
	}
	<-done
}
