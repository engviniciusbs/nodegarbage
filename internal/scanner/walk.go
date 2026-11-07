package scanner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/sys/windows"

	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

const cloudAttrs = windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS |
	windows.FILE_ATTRIBUTE_RECALL_ON_OPEN |
	windows.FILE_ATTRIBUTE_OFFLINE

// WalkScanner is a concurrent directory walker. Workers 0 means
// GOMAXPROCS*4.
type WalkScanner struct {
	FS      FS
	Workers int
}

// task is a directory still to be read. Its tree node is created by the
// worker after ReadDir so that denied dirs carry FlagDenied from the start
// and vanished dirs leave no node at all.
type task struct {
	parent tree.NodeID
	name   string
	path   string
	flags  tree.Flags
}

type walk struct {
	ctx context.Context
	fs  FS
	b   *tree.Builder
	p   *Progress

	mu      sync.Mutex
	cond    *sync.Cond
	stack   []task
	pending int
	stopped bool
	rootErr error
}

func joinPath(dir, name string) string {
	if strings.HasSuffix(dir, `\`) {
		return dir + name
	}
	return dir + `\` + name
}

func isVanished(err error) bool {
	return errors.Is(err, fs.ErrNotExist) ||
		errors.Is(err, windows.ERROR_PATH_NOT_FOUND) ||
		errors.Is(err, windows.ERROR_FILE_NOT_FOUND)
}

// Scan walks root and feeds b. It does NOT call b.Finish(): the caller must
// do so exactly once after Scan returns. Returns ctx.Err() on cancellation
// and an error wrapping ErrRootUnavailable if root cannot be read; failures
// in subdirectories never abort the scan.
func (s *WalkScanner) Scan(ctx context.Context, root string, b *tree.Builder, p *Progress) error {
	if p == nil {
		p = &Progress{}
	}
	n := s.Workers
	if n <= 0 {
		n = runtime.GOMAXPROCS(0) * 4
	}
	w := &walk{ctx: ctx, fs: s.FS, b: b, p: p, pending: 1}
	w.cond = sync.NewCond(&w.mu)
	w.stack = []task{{parent: tree.NoNode, name: root, path: root}}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			w.mu.Lock()
			w.stopped = true
			w.mu.Unlock()
			w.cond.Broadcast()
		case <-done:
		}
	}()

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			w.run(b.NewWorker())
		}()
	}
	wg.Wait()
	close(done)

	if w.rootErr != nil {
		return w.rootErr
	}
	return ctx.Err()
}

func (w *walk) stop() {
	w.stopped = true
	w.cond.Broadcast()
}

func (w *walk) run(tw *tree.Worker) {
	for {
		w.mu.Lock()
		for len(w.stack) == 0 && w.pending > 0 && !w.stopped {
			w.cond.Wait()
		}
		if w.stopped || len(w.stack) == 0 {
			w.mu.Unlock()
			return
		}
		t := w.stack[len(w.stack)-1]
		w.stack = w.stack[:len(w.stack)-1]
		w.mu.Unlock()

		var kids []task
		if w.ctx.Err() == nil {
			kids = w.process(tw, t)
		}

		w.mu.Lock()
		w.stack = append(w.stack, kids...)
		w.pending += len(kids) - 1
		if len(kids) > 0 || w.pending == 0 {
			w.cond.Broadcast()
		}
		w.mu.Unlock()
	}
}

// process reads one directory, registers it and its files in the builder and
// returns the subdirectories to descend into.
func (w *walk) process(tw *tree.Worker, t task) []task {
	path := t.path
	cur := path
	w.p.current.Store(&cur)

	ents, err := w.fs.ReadDir(path)
	flags := t.flags
	if err != nil {
		if t.parent == tree.NoNode {
			w.mu.Lock()
			w.rootErr = fmt.Errorf("%w: %v", ErrRootUnavailable, err)
			w.stop()
			w.mu.Unlock()
			return nil
		}
		if isVanished(err) {
			return nil
		}
		// Access denied or any other per-directory failure: record and go on.
		flags |= tree.FlagDenied
		w.b.AddDenied(path)
		w.p.denied.Add(1)
	}

	id := w.b.AddDir(t.parent, t.name, flags)
	w.p.dirs.Add(1)

	var (
		size, latest int64
		files        uint32
		kids         []task
	)
	for i := range ents {
		e := &ents[i]
		if e.IsDir {
			isCloud := e.Attrs&cloudAttrs != 0
			switch {
			case isCloud:
				// Placeholder dir: keep descending, just flag it.
				kids = append(kids, task{id, e.Name, joinPath(path, e.Name), tree.FlagCloud})
			case e.Attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0:
				w.b.AddDir(id, e.Name, tree.FlagLink)
				w.p.dirs.Add(1)
			default:
				kids = append(kids, task{id, e.Name, joinPath(path, e.Name), 0})
			}
			continue
		}
		sz := e.Size
		if e.Attrs&cloudAttrs != 0 || e.Attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			sz = 0 // not occupying local disk (or a link)
		}
		tw.AddFile(path, tree.FileEntry{Name: e.Name, Size: sz, ModTime: e.ModTime})
		size += sz
		files++
		if e.ModTime > latest {
			latest = e.ModTime
		}
	}
	w.b.SetOwnStats(id, size, files, latest)
	w.p.files.Add(int64(files))
	w.p.bytes.Add(size)
	return kids
}
