// Package scanner walks a volume concurrently and feeds a tree.Builder.
package scanner

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

// EngineWalk is the identifier of the portable concurrent directory walker.
const EngineWalk = "walk"

// ErrRootUnavailable is returned when the scan root cannot be read at all
// (drive removed, access denied on the root, ...).
var ErrRootUnavailable = errors.New("scanner: root unavailable")

// Progress holds live counters, updated with atomics. Zero value is ready.
type Progress struct {
	files, dirs, bytes, denied atomic.Int64
	current                    atomic.Pointer[string]
}

// ProgressSnapshot is a point-in-time copy of Progress.
type ProgressSnapshot struct {
	Files, Dirs, Bytes, Denied int64
	Current                    string
}

// Snapshot returns the current counters. Safe for concurrent use.
func (p *Progress) Snapshot() ProgressSnapshot {
	s := ProgressSnapshot{
		Files:  p.files.Load(),
		Dirs:   p.dirs.Load(),
		Bytes:  p.bytes.Load(),
		Denied: p.denied.Load(),
	}
	if c := p.current.Load(); c != nil {
		s.Current = *c
	}
	return s
}

// Scanner scans root and feeds b.
type Scanner interface {
	// Scan only feeds the builder; the caller must call b.Finish() exactly
	// once after Scan returns.
	Scan(ctx context.Context, root string, b *tree.Builder, p *Progress) error
}
