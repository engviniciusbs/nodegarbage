package cleaner

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
)

// Status is the outcome of one executed plan item.
type Status string

const (
	StatusOK      Status = "ok"
	StatusSkipped Status = "skipped"
	StatusError   Status = "error"
)

// Execution reasons (ItemResult.Reason). Recheck reasons (ReasonNotFound,
// ReasonChanged, ReasonProtected, ...) are also used, with StatusSkipped.
const (
	ReasonCanceled              = "canceled"                  // skipped: context cancelled before the item
	ReasonRecycleAborted        = "recycle_aborted"           // skipped: the shell operation was aborted (e.g. the user declined permanent deletion)
	ReasonPathTooLongForRecycle = "path_too_long_for_recycle" // error: the shell cannot recycle paths over MAX_PATH
	ReasonPartial               = "partial"                   // ok: some files were in use or denied (SkippedFiles)
	ReasonLocked                = "locked"                    // skipped: every file was in use or denied, nothing removed
	ReasonFailed                = "failed"                    // error: see Detail
	ReasonInvalidMode           = "invalid_mode"              // error: plan has an unknown Mode
	ReasonNoRecheck             = "no_recheck"                // skipped: Execute was given no recheck function
)

// MaxSkippedFiles caps ItemResult.SkippedFiles; SkippedCount keeps the total.
const MaxSkippedFiles = 100

// Errors a Deleter may return (wrapped or not); Execute maps them to reasons.
var (
	ErrRecycleAborted = errors.New("cleaner: recycle operation aborted")
	ErrPathTooLong    = errors.New("cleaner: path too long for the recycle bin")
	ErrNothingRemoved = errors.New("cleaner: every file was in use or access was denied")
	ErrReparseRoot    = errors.New("cleaner: item root is a junction or symbolic link")
)

// ItemResult is the outcome of one plan item. Path is the PlanItem's path.
// Freed is PlanItem.Size in recycle mode and the bytes of the files actually
// removed in permanent mode. SkippedFiles lists at most MaxSkippedFiles paths
// that could not be deleted (in use or denied); SkippedCount is their total.
// Detail carries the underlying error text for StatusError.
type ItemResult struct {
	Path         string
	Status       Status
	Reason       string
	Freed        int64
	SkippedFiles []string
	SkippedCount int
	Detail       string
}

// Deleter performs the deletions. Paths are canonical DOS paths without the
// \\?\ prefix. Implementations never traverse junctions, symbolic links or
// mount points: those are removed as links.
type Deleter interface {
	// Recycle moves one file or directory to the recycle bin.
	Recycle(path string) error
	// RemoveAll permanently deletes path and everything below it. Files in
	// use or denied are skipped and returned (uncapped); directories that
	// still hold them are kept. It returns ErrNothingRemoved when something
	// was skipped and nothing could be removed.
	RemoveAll(path string) (freed int64, skipped []string, err error)
	// RemoveContents is RemoveAll for everything below path, keeping path.
	// It returns ErrReparseRoot if path itself is a link.
	RemoveContents(path string) (freed int64, skipped []string, err error)
}

// Execute deletes the items of a redeemed plan, in order, honouring
// plan.Mode and each item's Class.Action. Before each item it checks ctx
// (once cancelled, this and the remaining items are skipped as canceled) and
// calls recheck; a non-empty reason skips the item without touching it.
// recheck is mandatory: when it is nil every item is skipped as no_recheck
// and the deleter is never called. A failing item never stops the others. onItem
// (may be nil) is called for every item, in order, right after its outcome
// is known. Execute must only ever be given a plan returned by
// Planner.Redeem.
func Execute(ctx context.Context, plan Plan, d Deleter, recheck func(PlanItem) string, onItem func(i int, r ItemResult)) []ItemResult {
	results := make([]ItemResult, len(plan.Items))
	for i, it := range plan.Items {
		r := ItemResult{Path: it.Path, Status: StatusSkipped, Reason: ReasonCanceled}
		if recheck == nil {
			r.Reason = ReasonNoRecheck
		} else if ctx.Err() == nil {
			r = executeItem(plan.Mode, it, d, recheck)
		}
		results[i] = r
		if onItem != nil {
			onItem(i, r)
		}
	}
	return results
}

func executeItem(mode Mode, it PlanItem, d Deleter, recheck func(PlanItem) string) ItemResult {
	r := ItemResult{Path: it.Path}
	action := it.Class.Action
	if action == "" {
		action = rules.ActionDeleteDir // as planned by Planner.Plan
	}
	switch {
	case mode != ModeRecycle && mode != ModePermanent:
		r.Status, r.Reason = StatusError, ReasonInvalidMode
		return r
	case action != rules.ActionDeleteDir && action != rules.ActionDeleteContents:
		r.Status, r.Reason = StatusError, ReasonInvalidAction
		return r
	}
	if reason := recheck(it); reason != "" {
		r.Status, r.Reason = StatusSkipped, reason
		return r
	}

	if mode == ModeRecycle {
		if action == rules.ActionDeleteDir {
			if err := d.Recycle(it.Path); err != nil {
				return failed(r, err)
			}
			r.Status, r.Freed = StatusOK, it.Size
			return r
		}
		return recycleContents(r, it, d)
	}

	var freed int64
	var skipped []string
	var err error
	if action == rules.ActionDeleteDir {
		freed, skipped, err = d.RemoveAll(it.Path)
	} else {
		freed, skipped, err = d.RemoveContents(it.Path)
	}
	r.Freed = freed
	setSkipped(&r, skipped)
	if err != nil {
		r = failed(r, err)
		r.Freed = freed
		return r
	}
	r.Status = StatusOK
	if r.SkippedCount > 0 {
		r.Reason = ReasonPartial
	}
	return r
}

// recycleContents recycles each direct child of the item. Children that
// fail are listed in SkippedFiles; an aborted operation stops the item.
func recycleContents(r ItemResult, it PlanItem, d Deleter) ItemResult {
	ents, err := os.ReadDir(it.Path)
	if err != nil {
		return failed(r, err)
	}
	var skipped []string
	var lastErr error
	recycled := 0
	for _, e := range ents {
		child := filepath.Join(it.Path, e.Name())
		err := d.Recycle(child)
		switch {
		case err == nil:
			recycled++
		case errors.Is(err, ErrRecycleAborted):
			r = failed(r, err)
			r.Freed = 0
			return r
		case errors.Is(err, fs.ErrNotExist):
			// Vanished meanwhile: nothing left to remove.
		default:
			skipped = append(skipped, child)
			lastErr = err
		}
	}
	setSkipped(&r, skipped)
	if recycled == 0 && lastErr != nil {
		return failed(r, lastErr)
	}
	r.Status, r.Freed = StatusOK, it.Size
	if r.SkippedCount > 0 {
		r.Reason = ReasonPartial
	}
	return r
}

func setSkipped(r *ItemResult, skipped []string) {
	r.SkippedCount = len(skipped)
	if len(skipped) > MaxSkippedFiles {
		skipped = skipped[:MaxSkippedFiles]
	}
	r.SkippedFiles = append([]string(nil), skipped...)
	if len(r.SkippedFiles) == 0 {
		r.SkippedFiles = nil
	}
}

// failed maps a deleter error to a status and reason. Freed is reset.
func failed(r ItemResult, err error) ItemResult {
	r.Freed = 0
	r.Detail = err.Error()
	switch {
	case errors.Is(err, ErrRecycleAborted):
		r.Status, r.Reason = StatusSkipped, ReasonRecycleAborted
	case errors.Is(err, ErrPathTooLong):
		r.Status, r.Reason = StatusError, ReasonPathTooLongForRecycle
	case errors.Is(err, ErrNothingRemoved):
		r.Status, r.Reason = StatusSkipped, ReasonLocked
	case errors.Is(err, ErrReparseRoot):
		r.Status, r.Reason = StatusSkipped, ReasonChanged
	case errors.Is(err, fs.ErrNotExist):
		r.Status, r.Reason = StatusSkipped, ReasonNotFound
	default:
		r.Status, r.Reason = StatusError, ReasonFailed
	}
	return r
}
