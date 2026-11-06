package tree

import (
	"container/heap"
	"sort"
)

// fileHeap is a min-heap by Size, keeping the N largest files.
type fileHeap []FileInfo

func (h fileHeap) Len() int           { return len(h) }
func (h fileHeap) Less(i, j int) bool { return h[i].Size < h[j].Size }
func (h fileHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *fileHeap) Push(x any)        { *h = append(*h, x.(FileInfo)) }
func (h *fileHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func (h *fileHeap) offer(f FileInfo, n int) {
	if n <= 0 {
		return
	}
	if len(*h) < n {
		heap.Push(h, f)
		return
	}
	if f.Size > (*h)[0].Size {
		(*h)[0] = f
		heap.Fix(h, 0)
	}
}

func (h fileHeap) sortedDesc() []FileInfo {
	out := make([]FileInfo, len(h))
	copy(out, h)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out
}
