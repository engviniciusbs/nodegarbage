package tree

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const maxDenied = 100

type Builder struct {
	topN int

	mu    sync.Mutex
	nodes []Node
	root  NodeID

	workersMu sync.Mutex
	workers   []*Worker

	deniedMu    sync.Mutex
	denied      []string
	deniedCount int
}

func NewBuilder(root string, topN int) *Builder {
	return &Builder{topN: topN, root: NoNode}
}

// AddDir is thread-safe. The root is created with AddDir(NoNode, root, 0).
func (b *Builder) AddDir(parent NodeID, name string, flags Flags) NodeID {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := NodeID(len(b.nodes))
	b.nodes = append(b.nodes, Node{Name: name, Parent: parent, Flags: flags})
	if parent == NoNode {
		b.root = id
	} else {
		b.nodes[parent].Children = append(b.nodes[parent].Children, id)
	}
	return id
}

// SetOwnStats is called once per dir by its owning worker. It takes the arena
// lock because concurrent AddDir calls may reallocate the slice.
func (b *Builder) SetOwnStats(id NodeID, size int64, files uint32, latest int64) {
	b.mu.Lock()
	n := &b.nodes[id]
	n.OwnSize, n.OwnFiles, n.OwnLatest = size, files, latest
	b.mu.Unlock()
}

func (b *Builder) AddDenied(path string) {
	b.deniedMu.Lock()
	defer b.deniedMu.Unlock()
	b.deniedCount++
	if len(b.denied) < maxDenied {
		b.denied = append(b.denied, path)
	}
}

// Worker is a per-goroutine accumulator (top-N files and extension stats).
type Worker struct {
	topN int
	heap fileHeap
	exts map[string]*ExtStat
}

func (b *Builder) NewWorker() *Worker {
	w := &Worker{topN: b.topN, exts: map[string]*ExtStat{}}
	b.workersMu.Lock()
	b.workers = append(b.workers, w)
	b.workersMu.Unlock()
	return w
}

func (w *Worker) AddFile(dirPath string, f FileEntry) {
	ext := strings.ToLower(filepath.Ext(f.Name))
	s := w.exts[ext]
	if s == nil {
		s = &ExtStat{Ext: ext}
		w.exts[ext] = s
	}
	s.Count++
	s.Bytes += f.Size
	// Avoid allocating the path unless the file makes the cut.
	if w.topN > 0 && (len(w.heap) < w.topN || f.Size > w.heap[0].Size) {
		w.heap.offer(FileInfo{Path: joinPath(dirPath, f.Name), Size: f.Size, ModTime: f.ModTime}, w.topN)
	}
}

// Finish merges workers and aggregates totals bottom-up. Call only after all
// goroutines using the builder have finished.
func (b *Builder) Finish() *Tree {
	t := &Tree{Root: b.root, Nodes: b.nodes, Denied: b.denied, DeniedCount: b.deniedCount}

	// Merge top-N.
	var merged fileHeap
	exts := map[string]*ExtStat{}
	for _, w := range b.workers {
		for _, f := range w.heap {
			merged.offer(f, b.topN)
		}
		for k, s := range w.exts {
			e := exts[k]
			if e == nil {
				e = &ExtStat{Ext: k}
				exts[k] = e
			}
			e.Count += s.Count
			e.Bytes += s.Bytes
		}
	}
	t.TopFiles = merged.sortedDesc()
	for _, e := range exts {
		t.Extensions = append(t.Extensions, *e)
	}
	sort.Slice(t.Extensions, func(i, j int) bool {
		a, c := t.Extensions[i], t.Extensions[j]
		if a.Bytes != c.Bytes {
			return a.Bytes > c.Bytes
		}
		return a.Ext < c.Ext
	})

	// Iterative post-order aggregation.
	if t.Root == NoNode {
		return t
	}
	order := make([]NodeID, 0, len(t.Nodes))
	stack := []NodeID{t.Root}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		order = append(order, id)
		stack = append(stack, t.Nodes[id].Children...)
	}
	for i := len(order) - 1; i >= 0; i-- {
		n := &t.Nodes[order[i]]
		n.TotalSize += n.OwnSize
		n.TotalFiles += n.OwnFiles
		if n.OwnLatest > n.TreeLatest {
			n.TreeLatest = n.OwnLatest
		}
		if n.Parent != NoNode {
			p := &t.Nodes[n.Parent]
			p.TotalSize += n.TotalSize
			p.TotalFiles += n.TotalFiles
			if n.TreeLatest > p.TreeLatest {
				p.TreeLatest = n.TreeLatest
			}
		}
	}
	return t
}
