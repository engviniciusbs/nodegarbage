// Package tree holds the in-memory aggregated directory tree built by the scanner.
package tree

import (
	"sort"
	"strings"
)

type NodeID int32

const NoNode NodeID = -1

type Flags uint8

const (
	FlagLink Flags = 1 << iota
	FlagCloud
	FlagDenied
)

type Node struct {
	Name       string
	Parent     NodeID
	Children   []NodeID
	OwnSize    int64
	TotalSize  int64
	OwnFiles   uint32
	TotalFiles uint32
	OwnLatest  int64
	TreeLatest int64 // max over the subtree
	Flags      Flags
}

type FileEntry struct {
	Name    string
	Size    int64
	ModTime int64
}

type FileInfo struct {
	Path    string
	Size    int64
	ModTime int64
}

// ExtStat: Ext is lowercase with a leading dot; "" means no extension.
type ExtStat struct {
	Ext   string
	Count uint64
	Bytes int64
}

type Tree struct {
	Root        NodeID
	Nodes       []Node
	TopFiles    []FileInfo // desc by Size, len <= topN
	Extensions  []ExtStat  // desc by Bytes
	Denied      []string
	DeniedCount int
}

// joinPath joins a directory path and a name with a single backslash.
func joinPath(dir, name string) string {
	if strings.HasSuffix(dir, `\`) {
		return dir + name
	}
	return dir + `\` + name
}

// Path returns the absolute path of the node, using "\" as separator.
func (t *Tree) Path(id NodeID) string {
	if id < 0 || int(id) >= len(t.Nodes) {
		return ""
	}
	var names []string
	for cur := id; cur != NoNode; cur = t.Nodes[cur].Parent {
		names = append(names, t.Nodes[cur].Name)
	}
	p := names[len(names)-1]
	for i := len(names) - 2; i >= 0; i-- {
		p = joinPath(p, names[i])
	}
	return p
}

// SortedChildren returns the children ordered by TotalSize descending.
func (t *Tree) SortedChildren(id NodeID) []NodeID {
	src := t.Nodes[id].Children
	out := make([]NodeID, len(src))
	copy(out, src)
	sort.SliceStable(out, func(i, j int) bool {
		return t.Nodes[out[i]].TotalSize > t.Nodes[out[j]].TotalSize
	})
	return out
}

// Find resolves a path case-insensitively; a trailing "\" is accepted.
func (t *Tree) Find(path string) (NodeID, bool) {
	if len(t.Nodes) == 0 {
		return NoNode, false
	}
	root := t.Nodes[t.Root].Name
	rootTrim := strings.TrimRight(root, `\`)
	pathTrim := strings.TrimRight(path, `\`)
	if len(pathTrim) < len(rootTrim) || !strings.EqualFold(pathTrim[:len(rootTrim)], rootTrim) {
		return NoNode, false
	}
	rest := pathTrim[len(rootTrim):]
	if rest != "" && rest[0] != 92 { // 92 = backslash
		return NoNode, false
	}
	cur := t.Root
	for _, comp := range strings.Split(rest, `\`) {
		if comp == "" {
			continue
		}
		found := NoNode
		for _, c := range t.Nodes[cur].Children {
			if strings.EqualFold(t.Nodes[c].Name, comp) {
				found = c
				break
			}
		}
		if found == NoNode {
			return NoNode, false
		}
		cur = found
	}
	return cur, true
}
