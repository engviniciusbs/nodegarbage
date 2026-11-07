package app

import (
	"errors"
	"strings"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

// LocateDTO é o caminho de nós (da primeira pasta abaixo da raiz até o alvo).
type LocateDTO struct {
	Volume string    `json:"volume"`
	Chain  []NodeDTO `json:"chain"`
}

var errNotLocated = errors.New("item não encontrado na última varredura")

// LocatePath localiza um caminho na última varredura e devolve a cadeia de
// nós, sem a raiz, para a UI abrir a pasta pai e destacar o item.
func (a *App) LocatePath(path string) (LocateDTO, error) {
	np := rules.NormalizePath(path)
	for _, s := range a.allSessions() {
		root := rules.NormalizePath(s.root)
		if np != root && !strings.HasPrefix(np, strings.TrimRight(root, `\`)+`\`) {
			continue
		}
		id, ok := s.tree.Find(strings.ReplaceAll(path, "/", `\`))
		if !ok || id == s.tree.Root || s.tree.IsRemoved(id) {
			continue
		}
		var rev []tree.NodeID
		for cur := id; cur != s.tree.Root && cur != tree.NoNode; cur = s.tree.Nodes[cur].Parent {
			rev = append(rev, cur)
		}
		chain := make([]NodeDTO, 0, len(rev))
		for i := len(rev) - 1; i >= 0; i-- {
			chain = append(chain, a.nodeDTO(s, rev[i]))
		}
		return LocateDTO{Volume: s.root, Chain: chain}, nil
	}
	return LocateDTO{}, errNotLocated
}
