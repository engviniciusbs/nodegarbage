package cleaner

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
)

// expiredGrace is how long an expired plan is remembered, so that Redeem can
// still answer ErrExpired (rather than ErrInvalidToken) after a purge.
const expiredGrace = 10 * time.Minute

type storedPlan struct {
	plan Plan
	hash [32]byte
}

// Planner validates requests into plans and redeems their tokens. It is safe
// for concurrent use.
type Planner struct {
	prot    *rules.Protector
	deps    Deps
	scanned func(volumeRoot string) bool
	now     func() time.Time

	mu    sync.Mutex
	plans map[string]storedPlan
}

// NewPlanner builds a planner. scanned reports whether a volume root (as
// returned by Deps.VolumeOf, e.g. `C:\`) was scanned in this session or has a
// saved snapshot. now is time.Now in production.
func NewPlanner(p *rules.Protector, d Deps, scanned func(volumeRoot string) bool, now func() time.Time) *Planner {
	if now == nil {
		now = time.Now
	}
	return &Planner{prot: p, deps: d, scanned: scanned, now: now, plans: map[string]storedPlan{}}
}

type candidate struct {
	idx  int
	norm string // NormalizePath(item.Path)
	vol  string
	item PlanItem
}

type refusal struct {
	idx int
	r   Refusal
}

type binInfo struct {
	remaining int64
	ok        bool // has a bin and it could be queried
}

// Plan validates every request, in input order, with these checks (first
// failing check wins): keep verdict, manual action, Canonicalize (not_found),
// Protector.Check on the canonical path, HasProtectedDescendant,
// ContainsGit unless the item's own action is delete_dir, scanned volume,
// and in recycle mode a usable recycle bin. Items that pass are then
// de-duplicated (an item equal to or inside another accepted item is refused
// as duplicate; the outer one stays) and, in recycle mode, accounted per
// volume in input order against the bin's remaining capacity.
//
// All checks and PlanItem.Path use the canonical path. An item without an
// action (unknown classification) is planned as delete_dir. fileCount (may be
// nil) is called with the requested path, as known to the scan tree.
//
// The returned Plan is a copy; mutating it does not affect the stored plan.
// When no item is accepted the plan has no Token and is not stored.
func (pl *Planner) Plan(reqs []Request, mode Mode, fileCount func(path string) uint32) (Plan, error) {
	if mode != ModeRecycle && mode != ModePermanent {
		return Plan{}, fmt.Errorf("%w: %q", ErrInvalidMode, mode)
	}
	var refused []refusal
	refuse := func(idx int, path, reason string) {
		refused = append(refused, refusal{idx, Refusal{Path: path, Reason: reason}})
	}
	bins := map[string]binInfo{}
	var cands []candidate

	for i, rq := range reqs {
		if reason, c := pl.check(rq, mode, bins); reason != "" {
			refuse(i, rq.Path, reason)
		} else {
			c.idx = i
			cands = append(cands, c)
		}
	}

	// De-duplicate: keep only outermost items.
	var kept []candidate
	for _, c := range cands {
		covered := false
		for _, k := range kept {
			if isUnder(c.norm, k.norm) {
				covered = true
				break
			}
		}
		if covered {
			refuse(c.idx, reqs[c.idx].Path, ReasonDuplicate)
			continue
		}
		next := kept[:0:0]
		for _, k := range kept {
			if isUnder(k.norm, c.norm) {
				refuse(k.idx, reqs[k.idx].Path, ReasonDuplicate)
			} else {
				next = append(next, k)
			}
		}
		kept = append(next, c)
	}
	sort.SliceStable(kept, func(a, b int) bool { return kept[a].idx < kept[b].idx })

	// Recycle-bin capacity, cumulative per volume in input order.
	var items []PlanItem
	used := map[string]int64{}
	var total int64
	for _, c := range kept {
		if mode == ModeRecycle {
			if used[c.vol]+c.item.Size > bins[c.vol].remaining {
				refuse(c.idx, reqs[c.idx].Path, ReasonExceedsRecycleBin)
				continue
			}
			used[c.vol] += c.item.Size
			if fileCount != nil && fileCount(reqs[c.idx].Path) >= ManyFilesThreshold {
				c.item.Warnings = append(c.item.Warnings, WarnManyFiles)
			}
		}
		total += c.item.Size
		items = append(items, c.item)
	}
	sort.SliceStable(refused, func(a, b int) bool { return refused[a].idx < refused[b].idx })

	id, err := randomHex(16)
	if err != nil {
		return Plan{}, err
	}
	p := Plan{ID: id, Mode: mode, Items: items, TotalBytes: total, ExpiresAt: pl.now().Add(TokenTTL)}
	for _, r := range refused {
		p.Refused = append(p.Refused, r.r)
	}

	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.purge(pl.now())
	if len(items) == 0 {
		return p, nil
	}
	if p.Token, err = randomHex(32); err != nil {
		return Plan{}, err
	}
	pl.plans[p.ID] = storedPlan{plan: copyPlan(p), hash: planHash(p)}
	return p, nil
}

// check runs the per-item checks; it returns a refusal reason, or "" and the
// candidate (without idx).
func (pl *Planner) check(rq Request, mode Mode, bins map[string]binInfo) (string, candidate) {
	cls := rq.Class
	if cls.Verdict == rules.Keep {
		return ReasonKeep, candidate{}
	}
	if cls.Action == rules.ActionManual {
		return ReasonManualAction, candidate{}
	}
	canon, err := pl.deps.Canonicalize(rq.Path)
	if err != nil || canon == "" {
		return ReasonNotFound, candidate{}
	}
	if pl.prot.Check(canon) != rules.ProtectNone {
		return ReasonProtected, candidate{}
	}
	if pl.prot.HasProtectedDescendant(canon) {
		return ReasonContainsProtected, candidate{}
	}
	if cls.Action != rules.ActionDeleteDir && pl.deps.ContainsGit(canon) {
		return ReasonContainsGit, candidate{}
	}
	vol := pl.deps.VolumeOf(canon)
	if vol == "" || pl.scanned == nil || !pl.scanned(vol) {
		return ReasonNotScanned, candidate{}
	}
	if mode == ModeRecycle {
		b, seen := bins[vol]
		if !seen {
			rem, has, err := pl.deps.RecycleBinRemaining(vol)
			b = binInfo{remaining: rem, ok: has && err == nil}
			bins[vol] = b
		}
		if !b.ok {
			return ReasonNoRecycleBin, candidate{}
		}
	}

	if cls.Action == "" {
		cls.Action = rules.ActionDeleteDir
	}
	norm := rules.NormalizePath(canon)
	item := PlanItem{Path: canon, Size: max(rq.Size, 0), Class: cls}
	if norm != rules.NormalizePath(rq.Path) {
		item.Warnings = append(item.Warnings, WarnCanonicalDiffers)
	}
	return "", candidate{norm: norm, vol: vol, item: item}
}

// Redeem returns the plan for planID if token matches and it has not
// expired. Every call consumes the plan, whatever the outcome: a used, wrong
// or expired token can never be retried. Unknown plans and wrong tokens yield
// ErrInvalidToken; expired plans ErrExpired.
func (pl *Planner) Redeem(planID, token string) (Plan, error) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	now := pl.now()
	s, ok := pl.plans[planID]
	delete(pl.plans, planID)
	pl.purge(now)
	if !ok || s.plan.Token == "" ||
		subtle.ConstantTimeCompare([]byte(s.plan.Token), []byte(token)) != 1 {
		return Plan{}, ErrInvalidToken
	}
	if now.After(s.plan.ExpiresAt) {
		return Plan{}, ErrExpired
	}
	if planHash(s.plan) != s.hash {
		return Plan{}, ErrInvalidToken
	}
	return copyPlan(s.plan), nil
}

// purge forgets plans expired for longer than expiredGrace. Caller holds mu.
func (pl *Planner) purge(now time.Time) {
	for id, s := range pl.plans {
		if now.After(s.plan.ExpiresAt.Add(expiredGrace)) {
			delete(pl.plans, id)
		}
	}
}

// planHash binds a plan's mode and accepted items (sorted normalized paths
// and their actions).
func planHash(p Plan) [32]byte {
	lines := make([]string, len(p.Items))
	for i, it := range p.Items {
		lines[i] = rules.NormalizePath(it.Path) + "\x00" + string(it.Class.Action)
	}
	sort.Strings(lines)
	return sha256.Sum256([]byte(string(p.Mode) + "\n" + strings.Join(lines, "\n")))
}

func copyPlan(p Plan) Plan {
	out := p
	out.Items = nil
	for _, it := range p.Items {
		it.Warnings = append([]string(nil), it.Warnings...)
		out.Items = append(out.Items, it)
	}
	out.Refused = append([]Refusal(nil), p.Refused...)
	return out
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", errors.Join(errors.New("cleaner: random source failed"), err)
	}
	return hex.EncodeToString(b), nil
}

// isUnder reports whether normalized path p equals base or lies inside it,
// comparing whole components.
func isUnder(p, base string) bool {
	if base == "" || !strings.HasPrefix(p, base) {
		return false
	}
	if len(p) == len(base) || strings.HasSuffix(base, `\`) {
		return true
	}
	return p[len(base)] == '\\'
}
