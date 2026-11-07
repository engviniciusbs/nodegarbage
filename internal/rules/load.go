package rules

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed builtin/*.yaml
var builtinFS embed.FS

// parse decodes a YAML document holding a top-level list of rules.
// Unknown keys are errors.
func parse(data []byte) ([]Rule, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var rs []Rule
	if err := dec.Decode(&rs); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return rs, nil
}

// LoadBuiltin loads the embedded rules; any invalid or duplicate rule is an error.
func LoadBuiltin() ([]Rule, error) {
	files, err := fs.Glob(builtinFS, "builtin/*.yaml")
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	out := []Rule{}
	seen := map[string]string{}
	for _, f := range files {
		data, err := builtinFS.ReadFile(f)
		if err != nil {
			return nil, err
		}
		rs, err := parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		for _, r := range rs {
			if err := Validate(r); err != nil {
				return nil, fmt.Errorf("%s: rule %q: %w", f, r.ID, err)
			}
			if prev, dup := seen[r.ID]; dup {
				return nil, fmt.Errorf("%s: duplicate rule id %q (also in %s)", f, r.ID, prev)
			}
			seen[r.ID] = f
			r.Builtin = true
			out = append(out, r)
		}
	}
	return out, nil
}

// LoadUserDir loads *.yaml / *.yml from dir in name order. Invalid rules and
// files are skipped and reported. A missing dir yields nothing.
func LoadUserDir(dir string) ([]Rule, []LoadProblem) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, []LoadProblem{{File: dir, Err: err.Error()}}
	}
	var names []string
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !e.IsDir() && (ext == ".yaml" || ext == ".yml") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var out []Rule
	var probs []LoadProblem
	seen := map[string]bool{}
	for _, n := range names {
		path := filepath.Join(dir, n)
		data, err := os.ReadFile(path)
		if err != nil {
			probs = append(probs, LoadProblem{File: path, Err: err.Error()})
			continue
		}
		rs, err := parse(data)
		if err != nil {
			probs = append(probs, LoadProblem{File: path, Err: err.Error()})
			continue
		}
		for _, r := range rs {
			if err := Validate(r); err != nil {
				probs = append(probs, LoadProblem{File: path, RuleID: r.ID, Err: err.Error()})
				continue
			}
			if seen[r.ID] {
				probs = append(probs, LoadProblem{File: path, RuleID: r.ID, Err: "duplicate rule id"})
				continue
			}
			seen[r.ID] = true
			out = append(out, r)
		}
	}
	return out, probs
}
