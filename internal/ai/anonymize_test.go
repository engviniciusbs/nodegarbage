package ai

import (
	"strings"
	"testing"
)

// USERPROFILE leaf differs from USERNAME (domain profile "jdoe.CORP"):
// neither may survive.
func TestAnonymizeProfileLeaf(t *testing.T) {
	a := NewUserAnonymizer([]string{"jdoe", "jdoe.CORP"}, nil)
	for _, in := range []string{
		`C:\Users\jdoe.CORP\src\app`,
		`D:\backup\jdoe.corp\x`,
		`D:\backup\jdoe\x`,
	} {
		got := a.Apply(in)
		if l := strings.ToLower(got); strings.Contains(l, "jdoe") || strings.Contains(l, "corp") {
			t.Errorf("Apply(%q) = %q leaks", in, got)
		}
	}
	if got := a.Apply(`C:\Users\jdoe.CORP\src`); got != `C:\Users\<user>\src` {
		t.Errorf("got %q", got)
	}
	if !a.Leaks(`x\JDOE.corp`) || !a.Leaks(`x\jdoe`) {
		t.Error("Leaks must cover every name")
	}
}

// The segment after X:\Users\ is always replaced, even for a name nobody
// passed in, except for the well-known shared folders.
func TestAnonymizeUsersSegment(t *testing.T) {
	a := NewUserAnonymizer([]string{"Vinicius"}, nil)
	cases := map[string]string{
		`C:\Users\maria\x`:              `C:\Users\<user>\x`,
		`d:\USERS\bo\AppData`:           `d:\USERS\<user>\AppData`,
		`C:\Users\maria`:                `C:\Users\<user>`,
		`C:\Users\Public\Documents`:     `C:\Users\Public\Documents`,
		`C:\Users\Default\NTUSER`:       `C:\Users\Default\NTUSER`,
		`C:\Users\All Users\x`:          `C:\Users\All Users\x`,
		`C:\Users\Default User\x`:       `C:\Users\Default User\x`,
		`C:\Users`:                      `C:\Users`,
		`C:\data\Users\maria`:           `C:\data\Users\maria`,
		`C:\Users\Vinicius\maria-notes`: `C:\Users\<user>\maria-notes`,
	}
	for in, want := range cases {
		if got := a.Apply(in); got != want {
			t.Errorf("Apply(%q) = %q, want %q", in, got, want)
		}
	}
}

// Names collected from the trees (the children of X:\Users) are anonymized
// everywhere, with the same >=4 substring / short whole-component rule.
func TestUserNamesFromTrees(t *testing.T) {
	tr := build(dir{`Users`, 0}, dir{`Users\maria`, 1}, dir{`Users\Public`, 1}, dir{`Users\bo`, 1}, dir{`other`, 1})
	names := UserNamesFromTrees(tr)
	got := strings.Join(names, ",")
	if got != "bo,maria" {
		t.Fatalf("names = %q", got)
	}
	a := NewUserAnonymizer(names, nil)
	if got := a.Apply(`D:\backup\maria-2024\bo\x`); got != `D:\backup\<user>-2024\<user>\x` {
		t.Errorf("got %q", got)
	}
	if got := a.Apply(`D:\bonus`); got != `D:\bonus` {
		t.Errorf("short name mangled: %q", got)
	}
}

// Shared folder names are never treated as user names.
func TestSharedFoldersNeverNames(t *testing.T) {
	a := NewUserAnonymizer([]string{"Public", "Default"}, nil)
	if got := a.Apply(`D:\Public\Default`); got != `D:\Public\Default` {
		t.Errorf("got %q", got)
	}
}
