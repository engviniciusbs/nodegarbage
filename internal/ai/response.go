package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

func decodeAnalysis(s string) (Analysis, bool) {
	var a Analysis
	if err := json.Unmarshal([]byte(s), &a); err != nil {
		return Analysis{}, false
	}
	if len(a.Items) == 0 && a.ReportMarkdown == "" {
		return Analysis{}, false
	}
	return a, true
}

// ParseAnalysis accepts plain JSON, JSON inside ``` fences, or text with a
// balanced {...} object somewhere in it.
func ParseAnalysis(text string) (Analysis, error) {
	t := strings.TrimSpace(text)
	if a, ok := decodeAnalysis(t); ok {
		return a, nil
	}
	if i := strings.Index(t, "```"); i >= 0 {
		rest := t[i+3:]
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 && !strings.ContainsAny(rest[:nl], "{}") {
			rest = rest[nl+1:] // drop the language tag line
		}
		if j := strings.Index(rest, "```"); j >= 0 {
			rest = rest[:j]
		}
		if a, ok := decodeAnalysis(strings.TrimSpace(rest)); ok {
			return a, nil
		}
	}
	for start := 0; start < len(t); {
		i := strings.IndexByte(t[start:], '{')
		if i < 0 {
			break
		}
		i += start
		end := balancedEnd(t, i)
		if end < 0 {
			break
		}
		if a, ok := decodeAnalysis(t[i : end+1]); ok {
			return a, nil
		}
		start = i + 1
	}
	return Analysis{}, fmt.Errorf("%w: no valid analysis object found", ErrInvalidResponse)
}

// balancedEnd returns the index of the brace closing the object that opens at
// s[start], honoring JSON strings and escapes; -1 if it never closes.
func balancedEnd(s string, start int) int {
	depth := 0
	inStr, esc := false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case inStr:
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
		case c == '"':
			inStr = true
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
