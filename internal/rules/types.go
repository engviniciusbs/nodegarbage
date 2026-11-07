// Package rules defines the YAML rule schema and its loaders.
package rules

import (
	"errors"
	"fmt"
)

type Verdict string

const (
	Safe    Verdict = "safe"
	Caution Verdict = "caution"
	Keep    Verdict = "keep"
)

func (v Verdict) rank() int {
	switch v {
	case Safe:
		return 0
	case Caution:
		return 1
	case Keep:
		return 2
	}
	return 2 // unknown values are treated as the most conservative
}

// MostConservative returns the more restrictive verdict: keep > caution > safe.
func MostConservative(a, b Verdict) Verdict {
	if b.rank() > a.rank() {
		return b
	}
	return a
}

// Category: dev-cache | build-artifact | pkg-cache | temp | browser-cache | system | user-data
type Category string

type Action string

const (
	ActionDeleteDir      Action = "delete_dir"
	ActionDeleteContents Action = "delete_contents"
	ActionManual         Action = "manual"
)

type Source string

const (
	SourceProtected Source = "protected"
	SourceRule      Source = "rule"
	SourceAI        Source = "ai"
	SourceUnknown   Source = "unknown"
)

// Match selects directories/files. Exactly one of Path, Dirname, Glob is set.
// Sibling (requires Dirname) is a simple glob matched with filepath.Match
// against the names in the parent directory.
type Match struct {
	Path          string `yaml:"path"`
	Dirname       string `yaml:"dirname"`
	Sibling       string `yaml:"sibling"`
	Glob          string `yaml:"glob"`
	OlderThanDays int    `yaml:"older_than_days"`
}

type Rule struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Category    Category `yaml:"category"`
	Match       Match    `yaml:"match"`
	Verdict     Verdict  `yaml:"verdict"`
	Regenerable string   `yaml:"regenerable"`
	Action      Action   `yaml:"action"`
	Command     string   `yaml:"command"`
	Explain     string   `yaml:"explain"`
	Builtin     bool     `yaml:"-"`
}

type LoadProblem struct{ File, RuleID, Err string }

var validCategories = map[Category]bool{
	"dev-cache": true, "build-artifact": true, "pkg-cache": true, "temp": true,
	"browser-cache": true, "system": true, "user-data": true,
}

// Validate checks the schema invariants of a single rule.
func Validate(r Rule) error {
	if r.ID == "" {
		return errors.New("id is required")
	}
	m := r.Match
	n := 0
	for _, s := range []string{m.Path, m.Dirname, m.Glob} {
		if s != "" {
			n++
		}
	}
	if n != 1 {
		return errors.New("match must set exactly one of path, dirname, glob")
	}
	if m.Sibling != "" && m.Dirname == "" {
		return errors.New("match.sibling requires match.dirname")
	}
	if m.OlderThanDays < 0 {
		return errors.New("match.older_than_days must be >= 0")
	}
	if m.OlderThanDays > 0 && m.Glob == "" {
		return errors.New("match.older_than_days is only valid with match.glob")
	}
	switch r.Verdict {
	case Safe, Caution, Keep:
	default:
		return fmt.Errorf("invalid verdict %q", r.Verdict)
	}
	if !validCategories[r.Category] {
		return fmt.Errorf("invalid category %q", r.Category)
	}
	switch r.Action {
	case ActionDeleteDir, ActionDeleteContents:
	case ActionManual:
		if r.Command == "" {
			return errors.New("action manual requires command")
		}
	default:
		return fmt.Errorf("invalid action %q", r.Action)
	}
	return nil
}
