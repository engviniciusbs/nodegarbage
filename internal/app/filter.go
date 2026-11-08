package app

import (
	"errors"
	"sort"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

// maxFiltered caps the items returned by ListFiltered (a var for tests).
var maxFiltered = 5000

// FilterDTO selects nodes across the whole volume: exactly one field is set.
type FilterDTO struct {
	Category string `json:"category"`
	RuleID   string `json:"ruleId"`
}

// FilteredDTO is the flattened filtered view. Count and TotalBytes cover
// every match, even when Items was capped (Truncated).
type FilteredDTO struct {
	Items      []NodeDTO `json:"items"`
	Count      int       `json:"count"`
	TotalBytes int64     `json:"totalBytes"`
	Truncated  bool      `json:"truncated"`
}

var errBadFilter = errors.New("informe exatamente um filtro: categoria ou regra")

// categoryKey is the category a classification is reported under in the
// summary: AI-only classifications have no category and go under "ai".
func categoryKey(c rules.Classification) string {
	if c.Category == "" && c.Source == rules.SourceAI {
		return "ai"
	}
	return string(c.Category)
}

func filterMatch(c rules.Classification, f FilterDTO) bool {
	if c.Verdict == "" {
		return false
	}
	if f.RuleID != "" {
		return c.RuleID == f.RuleID
	}
	return categoryKey(c) == f.Category
}

// ListFiltered devolve, do volume inteiro, os nós de topo cuja classificação
// efetiva casa com o filtro (sem repetir nós dentro de outro que já casou),
// por tamanho decrescente.
func (a *App) ListFiltered(volume string, f FilterDTO) (FilteredDTO, error) {
	if (f.Category == "") == (f.RuleID == "") {
		return FilteredDTO{}, errBadFilter
	}
	s, err := a.session(volume)
	if err != nil {
		return FilteredDTO{}, err
	}
	out := FilteredDTO{Items: []NodeDTO{}}
	t := s.tree
	if len(t.Nodes) == 0 {
		return out, nil
	}
	var hits []tree.NodeID
	stack := []tree.NodeID{t.Root}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if id != t.Root && filterMatch(a.effective(s, id), f) {
			hits = append(hits, id) // a match covers its subtree: never descend
			continue
		}
		for _, k := range t.Nodes[id].Children {
			if !t.IsRemoved(k) {
				stack = append(stack, k)
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		si, sj := t.Nodes[hits[i]].TotalSize, t.Nodes[hits[j]].TotalSize
		if si != sj {
			return si > sj
		}
		return t.Path(hits[i]) < t.Path(hits[j])
	})
	out.Count = len(hits)
	for _, id := range hits {
		out.TotalBytes += t.Nodes[id].TotalSize
	}
	if len(hits) > maxFiltered {
		hits, out.Truncated = hits[:maxFiltered], true
	}
	for _, id := range hits {
		d := a.nodeDTO(s, id)
		if p := t.Nodes[id].Parent; p != tree.NoNode {
			d.Location = t.Path(p)
		}
		out.Items = append(out.Items, d)
	}
	return out, nil
}
