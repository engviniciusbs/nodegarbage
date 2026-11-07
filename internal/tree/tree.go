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

// Clone returns a copy that can be edited with Remove and RemoveContents
// without disturbing readers of t: the Nodes slice is copied, while Children
// and TopFiles slices stay shared because the editing methods always replace
// them and never modify them in place.
func (t *Tree) Clone() *Tree {
	c := *t
	c.Nodes = append([]Node(nil), t.Nodes...)
	return &c
}

// IsRemoved reports whether id was detached by Remove (or is out of range).
// The root is never removed.
func (t *Tree) IsRemoved(id NodeID) bool {
	if id < 0 || int(id) >= len(t.Nodes) {
		return true
	}
	return id != t.Root && t.Nodes[id].Parent == NoNode
}

// subtractAncestors lowers the totals of every ancestor of id (TreeLatest is
// left as is).
func (t *Tree) subtractAncestors(id NodeID, size int64, files uint32) {
	for cur := t.Nodes[id].Parent; cur != NoNode; cur = t.Nodes[cur].Parent {
		n := &t.Nodes[cur]
		n.TotalSize -= size
		if n.TotalFiles >= files {
			n.TotalFiles -= files
		} else {
			n.TotalFiles = 0
		}
	}
}

// dropTopFiles removes top files located under dir (a path, no trailing "\").
func (t *Tree) dropTopFiles(dir string) {
	prefix := strings.ToLower(strings.TrimRight(dir, `\`)) + `\`
	var out []FileInfo
	changed := false
	for _, f := range t.TopFiles {
		if strings.HasPrefix(strings.ToLower(f.Path), prefix) {
			changed = true
			continue
		}
		out = append(out, f)
	}
	if changed {
		t.TopFiles = out
	}
}

// Remove detaches the node from its parent and subtracts its totals from all
// ancestors. The node is marked removed (Parent = NoNode, unreachable from
// the root); ids of other nodes are unchanged. Removing the root, an
// out-of-range id or an already removed node does nothing. Remove replaces
// the parent's Children slice instead of editing it, so it is safe on a Clone.
func (t *Tree) Remove(id NodeID) {
	if id == t.Root || t.IsRemoved(id) {
		return
	}
	path := t.Path(id)
	n := t.Nodes[id]
	t.subtractAncestors(id, n.TotalSize, n.TotalFiles)
	p := &t.Nodes[n.Parent]
	kids := make([]NodeID, 0, len(p.Children))
	for _, c := range p.Children {
		if c != id {
			kids = append(kids, c)
		}
	}
	p.Children = kids
	t.Nodes[id].Parent = NoNode
	t.dropTopFiles(path)
}

// RemoveContents empties the node but keeps it: its children are detached
// (marked removed), its own and total size/file counts become zero and the
// ancestors are adjusted.
func (t *Tree) RemoveContents(id NodeID) {
	if id < 0 || int(id) >= len(t.Nodes) || t.IsRemoved(id) {
		return
	}
	path := t.Path(id)
	n := t.Nodes[id]
	t.subtractAncestors(id, n.TotalSize, n.TotalFiles)
	for _, c := range n.Children {
		t.Nodes[c].Parent = NoNode
	}
	t.Nodes[id].Children = nil
	t.Nodes[id].OwnSize, t.Nodes[id].OwnFiles = 0, 0
	t.Nodes[id].TotalSize, t.Nodes[id].TotalFiles = 0, 0
	t.dropTopFiles(path)
}
