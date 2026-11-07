package rules

import "strings"

// Env looks up an environment variable (os.Getenv in production). Names are
// expected to be case-insensitive, as on Windows.
type Env func(key string) string

// NormalizePath returns the canonical comparison form of a Windows path:
// "/" becomes "\", the \\?\ and \\.\ prefixes (and their UNC\ forms) are
// removed, repeated separators are collapsed, "." and ".." components are
// resolved, trailing dots/spaces of each component are trimmed, there is no
// trailing separator (except for a volume root such as "c:\") and the result is
// lowercase. It is purely lexical and never touches the disk.
func NormalizePath(p string) string {
	p = strings.ReplaceAll(p, "/", `\`)
	for _, pre := range []string{`\\?\`, `\\.\`} {
		if len(p) >= len(pre)+4 && strings.EqualFold(p[len(pre):len(pre)+4], `UNC\`) && strings.HasPrefix(p, pre) {
			p = `\\` + p[len(pre)+4:]
			break
		}
		if strings.HasPrefix(p, pre) {
			p = p[len(pre):]
			break
		}
	}
	lead := ""
	if strings.HasPrefix(p, `\\`) {
		lead, p = `\\`, p[2:]
	} else if strings.HasPrefix(p, `\`) {
		lead, p = `\`, p[1:]
	}

	var parts []string
	head := 0 // components ".." may not climb above
	for i, c := range strings.Split(p, `\`) {
		switch c {
		case "", ".":
			continue
		case "..":
			if len(parts) > head {
				parts = parts[:len(parts)-1]
			}
			continue
		}
		// Win32 ignores trailing dots and spaces of a component:
		// `C:\Windows.\x` and `C:\Windows \x` both name C:\Windows\x.
		if c = strings.TrimRight(c, ". "); c == "" {
			continue
		}
		parts = append(parts, c)
		if i == 0 && lead == "" && len(c) == 2 && c[1] == ':' {
			head = 1 // drive letter
		}
		if lead == `\\` && len(parts) <= 2 {
			head = len(parts) // \\server\share
		}
	}
	out := lead + strings.Join(parts, `\`)
	if len(out) == 2 && out[1] == ':' {
		out += `\`
	}
	return strings.ToLower(out)
}

// isUnder reports whether normalized path p equals base or lies inside it,
// comparing whole components (c:\windows2 is not under c:\windows).
func isUnder(p, base string) bool {
	if base == "" || !strings.HasPrefix(p, base) {
		return false
	}
	if len(p) == len(base) || strings.HasSuffix(base, `\`) {
		return true
	}
	return p[len(base)] == '\\'
}

// joinNorm appends a (lowercased) name to a normalized directory path.
func joinNorm(dir, name string) string {
	name = strings.ToLower(name)
	if strings.HasSuffix(dir, `\`) {
		return dir + name
	}
	return dir + `\` + name
}

// expandEnv replaces %NAME% tokens using env. It fails if any referenced
// variable is empty or missing, so that such an entry can be skipped instead
// of collapsing into a dangerously short path. A "%" without a closing "%" is
// kept literally.
func expandEnv(s string, env Env) (string, bool) {
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '%')
		if i < 0 {
			b.WriteString(s)
			return b.String(), true
		}
		j := strings.IndexByte(s[i+1:], '%')
		if j < 0 {
			b.WriteString(s)
			return b.String(), true
		}
		name := s[i+1 : i+1+j]
		v := env(name)
		if v == "" {
			v = env(strings.ToUpper(name))
		}
		if v == "" {
			return "", false
		}
		b.WriteString(s[:i])
		b.WriteString(v)
		s = s[i+2+j:]
	}
}

// expandNorm expands env vars and normalizes; ok is false when the result
// must be ignored (empty variable or empty path).
func expandNorm(s string, env Env) (string, bool) {
	e, ok := expandEnv(s, env)
	if !ok {
		return "", false
	}
	n := NormalizePath(e)
	if n == "" {
		return "", false
	}
	return n, true
}

// IsAbsoluteEntry reports whether s, after %VAR% expansion with env (every
// referenced variable must be defined), is an absolute drive path (`X:\...`)
// or a UNC path with server and share (`\server\share[\...]`). Drive-relative
// (`C:`, `C:foo`), rooted (`\x`), relative and device (`\?\`, `\.\`) forms
// are rejected. "/" counts as "\".
func IsAbsoluteEntry(s string, env Env) bool {
	e, ok := expandEnv(strings.TrimSpace(s), env)
	if !ok {
		return false
	}
	e = strings.ReplaceAll(e, "/", `\`)
	if len(e) >= 3 && e[1] == ':' && e[2] == '\\' {
		c := e[0] | 0x20
		return c >= 'a' && c <= 'z'
	}
	if !strings.HasPrefix(e, `\`) || strings.HasPrefix(e, `\?\`) || strings.HasPrefix(e, `\.\`) {
		return false
	}
	parts := strings.Split(e[2:], `\`)
	return len(parts) >= 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != ""
}
