// Package cleaner validates deletion requests and turns them into single-use
// plans. Only a plan redeemed with its token may be executed: the planner is
// the gate that guarantees nothing protected is ever deleted.
package cleaner

import (
	"errors"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
)

// Mode is how accepted items are deleted.
type Mode string

const (
	ModeRecycle   Mode = "recycle"
	ModePermanent Mode = "permanent"
)

// Request is one user-selected path, with the size and classification the
// UI showed for it.
type Request struct {
	Path  string
	Size  int64
	Class rules.Classification
}

// PlanItem is an accepted item. Path is the canonical path (resolved through
// junctions, symlinks and 8.3 names), which is what will be deleted.
type PlanItem struct {
	Path     string
	Size     int64
	Class    rules.Classification
	Warnings []string
}

// Refusal is a rejected request. Path is the path as requested.
type Refusal struct {
	Path, Reason string
}

// Refusal reasons.
const (
	ReasonNotFound          = "not_found"
	ReasonProtected         = "protected"
	ReasonContainsProtected = "contains_protected"
	ReasonContainsGit       = "contains_git"
	ReasonNotScanned        = "not_scanned"
	ReasonNoRecycleBin      = "no_recycle_bin"
	ReasonExceedsRecycleBin = "exceeds_recycle_bin"
	ReasonManualAction      = "manual_action"
	ReasonKeep              = "keep"
	// ReasonDuplicate: the item equals or lies inside another accepted item,
	// which already covers it.
	ReasonDuplicate = "duplicate"
	// ReasonInvalidAction: Class.Action is not "", delete_dir,
	// delete_contents or manual.
	ReasonInvalidAction = "invalid_action"
	// ReasonInvalidSize: the request has a negative Size.
	ReasonInvalidSize = "invalid_size"
	// ReasonChanged (Recheck): the item no longer canonicalizes to the
	// planned path, e.g. it was replaced by a junction.
	ReasonChanged = "changed"
)

// Item warnings.
const (
	WarnManyFiles = "many_files" // ≥ ManyFilesThreshold files in recycle mode
	// WarnCanonicalDiffers: the canonical path is not the requested one
	// (e.g. the request went through a junction).
	WarnCanonicalDiffers = "canonical_differs"
)

const (
	// ManyFilesThreshold is the file count from which recycling is slow.
	ManyFilesThreshold = 20000
	// TokenTTL is how long a plan's token can be redeemed.
	TokenTTL = 5 * time.Minute
)

// Plan is the validated result of Planner.Plan. A plan with no accepted
// items has an empty Token, is not stored and cannot be redeemed.
type Plan struct {
	ID         string
	Mode       Mode
	Items      []PlanItem
	Refused    []Refusal
	TotalBytes int64
	Token      string
	ExpiresAt  time.Time
}

// Deps is the file-system access the planner needs (real implementation in
// deps_windows.go; fakes in tests).
type Deps interface {
	// Canonicalize returns the final DOS path of an existing item (no \\?\
	// prefix). A missing item yields an error wrapping fs.ErrNotExist.
	Canonicalize(path string) (string, error)
	// ContainsGit reports whether some entry named .git exists below path.
	ContainsGit(path string) bool
	// RecycleBinRemaining returns the free recycle-bin capacity of a volume
	// root such as `C:\`; hasBin is false when the volume has no usable bin.
	RecycleBinRemaining(volumeRoot string) (remaining int64, hasBin bool, err error)
	// VolumeOf returns the volume root of a path, e.g. `C:\`.
	VolumeOf(path string) string
}

var (
	ErrInvalidToken = errors.New("cleaner: invalid or already used token")
	ErrExpired      = errors.New("cleaner: plan expired")
	ErrInvalidMode  = errors.New("cleaner: invalid mode")
)
