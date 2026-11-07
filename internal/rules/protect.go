package rules

import (
	"strings"
	"sync"
)

// ProtectKind says how a path is covered by the hard-coded protection list.
type ProtectKind int

const (
	ProtectNone    ProtectKind = iota
	ProtectExact               // only the path itself is protected
	ProtectSubtree             // the path and everything inside it
)

func (k ProtectKind) String() string {
	switch k {
	case ProtectExact:
		return "exact"
	case ProtectSubtree:
		return "subtree"
	}
	return "none"
}

// Fixed protection list (spec §4). No YAML rule can override it.
var (
	protectedSubtreeEnv = []string{
		`%SystemRoot%`,
		`%ProgramFiles%`,
		`%ProgramFiles(x86)%`,
		`%ProgramData%\Microsoft`,
	}
	// Exceptions carved out of protectedSubtreeEnv (not out of user extras).
	protectedExceptionEnv = []string{`%SystemRoot%\Temp`}
	protectedRootSubtree  = []string{`System Volume Information`, `$Recycle.Bin`}
	protectedRootExact    = []string{`pagefile.sys`, `hiberfil.sys`, `swapfile.sys`}
	protectedExactEnv     = []string{
		`%USERPROFILE%`,
		`%USERPROFILE%\Documents`,
		`%USERPROFILE%\Desktop`,
		`%USERPROFILE%\Pictures`,
		`%USERPROFILE%\Videos`,
		`%USERPROFILE%\Music`,
		`%USERPROFILE%\Downloads`,
		`%USERPROFILE%\AppData`,
		`%USERPROFILE%\AppData\Local`,
		`%USERPROFILE%\AppData\Roaming`,
		`%USERPROFILE%\AppData\LocalLow`,
	}
)

// Protector answers whether a path is protected. All stored paths are in
// NormalizePath form. It is safe for concurrent use; only SetExtra mutates it.
type Protector struct {
	env        Env
	mu         sync.RWMutex // guards extra
	subtree    []string     // builtin subtrees, subject to exceptions
	exceptions []string
	extra      []string // user subtrees, no exceptions
	exact      map[string]bool
}

// NewProtector builds the protection list. Entries whose environment variable
// is empty or missing are skipped. volumeRoots are paths like `C:\`; extra are
// user-defined subtree paths (env vars allowed).
func NewProtector(env Env, volumeRoots []string, extra []string) *Protector {
	p := &Protector{env: env, exact: map[string]bool{}}
	add := func(dst *[]string, entries []string) {
		for _, e := range entries {
			if n, ok := expandNorm(e, env); ok {
				*dst = append(*dst, n)
			}
		}
	}
	add(&p.subtree, protectedSubtreeEnv)
	add(&p.exceptions, protectedExceptionEnv)
	add(&p.extra, extra)
	var exact []string
	add(&exact, protectedExactEnv)
	for _, r := range volumeRoots {
		root := NormalizePath(r)
		if root == "" {
			continue
		}
		exact = append(exact, root)
		for _, name := range protectedRootExact {
			exact = append(exact, joinNorm(root, name))
		}
		for _, name := range protectedRootSubtree {
			p.subtree = append(p.subtree, joinNorm(root, name))
		}
	}
	for _, e := range exact {
		p.exact[e] = true
	}
	return p
}

// Check classifies path. ProtectSubtree is also returned for any path inside
// a protected subtree (including any path with a ".git" component).
//
// Check is purely lexical: it cannot see through 8.3 short names, junctions,
// symlinks or mount points. Callers about to delete files must pass the
// canonical path of the opened item (GetFinalPathNameByHandle output), not a
// user- or AI-supplied string.
func (p *Protector) Check(path string) ProtectKind {
	return p.checkNorm(NormalizePath(path))
}

func (p *Protector) checkNorm(n string) ProtectKind {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if n == "" {
		return ProtectNone
	}
	if hasGitComponent(n) || p.inSubtree(n) {
		return ProtectSubtree
	}
	if p.exact[n] {
		return ProtectExact
	}
	return ProtectNone
}

// inSubtree reports whether n lies in a builtin (minus exceptions) or extra subtree.
func (p *Protector) inSubtree(n string) bool {
	for _, e := range p.extra {
		if isUnder(n, e) {
			return true
		}
	}
	for _, s := range p.subtree {
		if isUnder(n, s) && !p.inException(n) {
			return true
		}
	}
	return false
}

func (p *Protector) inException(n string) bool {
	for _, x := range p.exceptions {
		if isUnder(n, x) {
			return true
		}
	}
	return false
}

// exceptionBelow reports whether a protection exception lies strictly below n.
func (p *Protector) exceptionBelow(n string) bool {
	for _, x := range p.exceptions {
		if x != n && isUnder(x, n) {
			return true
		}
	}
	return false
}

// HasProtectedDescendant reports whether some protected path (exact or
// subtree, excluding .git) lies strictly below path. A path that is itself
// inside a protected subtree also returns true, since everything below it is
// protected.
func (p *Protector) HasProtectedDescendant(path string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	n := NormalizePath(path)
	if n == "" {
		return false
	}
	if p.inSubtree(n) {
		return true
	}
	below := func(e string) bool { return e != n && isUnder(e, n) }
	for _, e := range p.extra {
		if below(e) {
			return true
		}
	}
	for _, s := range p.subtree {
		if below(s) {
			return true
		}
	}
	for e := range p.exact {
		if below(e) {
			return true
		}
	}
	return false
}

func hasGitComponent(n string) bool {
	for _, c := range strings.Split(n, `\`) {
		if c == ".git" {
			return true
		}
	}
	return false
}

// SetExtra replaces the user-defined protected subtrees (env vars allowed;
// entries that expand to nothing are skipped). It takes effect immediately and
// is safe to call concurrently with Check and HasProtectedDescendant.
func (p *Protector) SetExtra(extra []string) {
	var out []string
	env := p.env
	if env == nil {
		env = func(string) string { return "" }
	}
	for _, e := range extra {
		if n, ok := expandNorm(e, env); ok {
			out = append(out, n)
		}
	}
	p.mu.Lock()
	p.extra = out
	p.mu.Unlock()
}

// ValidExtra reports whether an extra protected entry is usable with this
// protector's environment (see IsAbsoluteEntry).
func (p *Protector) ValidExtra(s string) bool { return IsAbsoluteEntry(s, p.env) }
