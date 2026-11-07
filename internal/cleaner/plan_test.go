package cleaner

import (
	"errors"
	"io/fs"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
)

type fakeBin struct {
	remaining int64
	has       bool
	err       error
}

type fakeDeps struct {
	canon map[string]string // lowercase request path -> canonical; missing -> not found
	git   map[string]bool   // lowercase canonical path -> contains .git
	bins  map[string]fakeBin
	calls map[string]int // ContainsGit calls per path
}

func newFakeDeps() *fakeDeps {
	return &fakeDeps{
		canon: map[string]string{},
		git:   map[string]bool{},
		bins:  map[string]fakeBin{`C:\`: {remaining: 1 << 40, has: true}, `D:\`: {remaining: 1 << 40, has: true}},
		calls: map[string]int{},
	}
}

// exist registers paths that canonicalize to themselves.
func (d *fakeDeps) exist(paths ...string) {
	for _, p := range paths {
		d.canon[strings.ToLower(p)] = p
	}
}

func (d *fakeDeps) Canonicalize(p string) (string, error) {
	c, ok := d.canon[strings.ToLower(p)]
	if !ok {
		return "", fs.ErrNotExist
	}
	return c, nil
}

func (d *fakeDeps) ContainsGit(p string) bool {
	d.calls[strings.ToLower(p)]++
	return d.git[strings.ToLower(p)]
}

func (d *fakeDeps) RecycleBinRemaining(root string) (int64, bool, error) {
	b, ok := d.bins[root]
	if !ok {
		return 0, false, nil
	}
	return b.remaining, b.has, b.err
}

func (d *fakeDeps) VolumeOf(p string) string {
	if len(p) >= 2 && p[1] == ':' {
		return strings.ToUpper(p[:1]) + `:\`
	}
	return ""
}

var testEnv = rules.Env(func(k string) string {
	switch strings.ToUpper(k) {
	case "SYSTEMROOT":
		return `C:\Windows`
	case "PROGRAMFILES":
		return `C:\Program Files`
	case "USERPROFILE":
		return `C:\Users\me`
	}
	return ""
})

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newTestPlanner(d Deps, scannedVols ...string) (*Planner, *clock) {
	if len(scannedVols) == 0 {
		scannedVols = []string{`C:\`, `D:\`}
	}
	set := map[string]bool{}
	for _, v := range scannedVols {
		set[v] = true
	}
	c := &clock{t: time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
	p := rules.NewProtector(testEnv, []string{`C:\`, `D:\`}, nil)
	return NewPlanner(p, d, func(v string) bool { return set[v] }, c.now), c
}

var (
	clsSafeDir      = rules.Classification{RuleID: "node-modules", Verdict: rules.Safe, Action: rules.ActionDeleteDir, Source: rules.SourceRule}
	clsSafeContents = rules.Classification{RuleID: "temp", Verdict: rules.Safe, Action: rules.ActionDeleteContents, Source: rules.SourceRule}
	clsUnknown      = rules.Classification{Source: rules.SourceUnknown}
)

func refusalReason(p Plan, path string) string {
	for _, r := range p.Refused {
		if strings.EqualFold(r.Path, path) {
			return r.Reason
		}
	}
	return ""
}

func accepted(p Plan, path string) bool {
	for _, it := range p.Items {
		if strings.EqualFold(it.Path, path) {
			return true
		}
	}
	return false
}

func TestRefusalReasons(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(d *fakeDeps) []string // returns scanned volumes override (nil = default)
		req    Request
		mode   Mode
		reason string
	}{
		{"keep", nil, Request{Path: `C:\x\keep`, Size: 1, Class: rules.Classification{Verdict: rules.Keep, Action: rules.ActionDeleteDir}}, ModeRecycle, ReasonKeep},
		{"manual", nil, Request{Path: `C:\x\manual`, Size: 1, Class: rules.Classification{Verdict: rules.Caution, Action: rules.ActionManual}}, ModeRecycle, ReasonManualAction},
		{"not_found", nil, Request{Path: `C:\x\gone`, Size: 1, Class: clsSafeDir}, ModeRecycle, ReasonNotFound},
		{"protected", func(d *fakeDeps) []string { d.exist(`C:\Windows\Temp2`); return nil },
			Request{Path: `C:\Windows\Temp2`, Size: 1, Class: clsSafeDir}, ModeRecycle, ReasonProtected},
		{"protected_exact", func(d *fakeDeps) []string { d.exist(`C:\Users\me\Downloads`); return nil },
			Request{Path: `C:\Users\me\Downloads`, Size: 1, Class: clsSafeContents}, ModeRecycle, ReasonProtected},
		{"contains_protected", func(d *fakeDeps) []string { d.exist(`C:\Users`); return nil },
			Request{Path: `C:\Users`, Size: 1, Class: clsSafeDir}, ModeRecycle, ReasonContainsProtected},
		{"contains_git", func(d *fakeDeps) []string {
			d.exist(`C:\x\repo`)
			d.git[`c:\x\repo`] = true
			return nil
		}, Request{Path: `C:\x\repo`, Size: 1, Class: clsSafeContents}, ModeRecycle, ReasonContainsGit},
		{"not_scanned", func(d *fakeDeps) []string { d.exist(`D:\cache`); return []string{`C:\`} },
			Request{Path: `D:\cache`, Size: 1, Class: clsSafeDir}, ModePermanent, ReasonNotScanned},
		{"no_recycle_bin", func(d *fakeDeps) []string {
			d.exist(`D:\cache`)
			d.bins[`D:\`] = fakeBin{has: false}
			return nil
		}, Request{Path: `D:\cache`, Size: 1, Class: clsSafeDir}, ModeRecycle, ReasonNoRecycleBin},
		{"recycle_bin_query_error", func(d *fakeDeps) []string {
			d.exist(`D:\cache`)
			d.bins[`D:\`] = fakeBin{remaining: 100, has: true, err: errors.New("boom")}
			return nil
		}, Request{Path: `D:\cache`, Size: 1, Class: clsSafeDir}, ModeRecycle, ReasonNoRecycleBin},
		{"exceeds_recycle_bin", func(d *fakeDeps) []string {
			d.exist(`C:\big`)
			d.bins[`C:\`] = fakeBin{remaining: 10, has: true}
			return nil
		}, Request{Path: `C:\big`, Size: 11, Class: clsSafeDir}, ModeRecycle, ReasonExceedsRecycleBin},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newFakeDeps()
			var vols []string
			if tc.setup != nil {
				vols = tc.setup(d)
			}
			pl, _ := newTestPlanner(d, vols...)
			p, err := pl.Plan([]Request{tc.req}, tc.mode, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Items) != 0 {
				t.Fatalf("item accepted: %+v", p.Items)
			}
			if got := refusalReason(p, tc.req.Path); got != tc.reason {
				t.Fatalf("reason = %q, want %q (refused %+v)", got, tc.reason, p.Refused)
			}
		})
	}
}

func TestCheckOrder(t *testing.T) {
	// keep wins over manual and over not_found.
	d := newFakeDeps()
	pl, _ := newTestPlanner(d)
	p, _ := pl.Plan([]Request{{Path: `C:\nope`, Class: rules.Classification{Verdict: rules.Keep, Action: rules.ActionManual}}}, ModeRecycle, nil)
	if got := refusalReason(p, `C:\nope`); got != ReasonKeep {
		t.Fatalf("got %q", got)
	}
	// manual wins over not_found.
	p, _ = pl.Plan([]Request{{Path: `C:\nope`, Class: rules.Classification{Verdict: rules.Caution, Action: rules.ActionManual}}}, ModeRecycle, nil)
	if got := refusalReason(p, `C:\nope`); got != ReasonManualAction {
		t.Fatalf("got %q", got)
	}
}

func TestJunctionIntoProtectedRefused(t *testing.T) {
	d := newFakeDeps()
	d.canon[`d:\link`] = `C:\Windows\System32`
	pl, _ := newTestPlanner(d)
	p, err := pl.Plan([]Request{{Path: `D:\link`, Size: 5, Class: clsSafeDir}}, ModePermanent, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := refusalReason(p, `D:\link`); got != ReasonProtected {
		t.Fatalf("reason = %q, want protected", got)
	}
}

func TestCanonicalPathUsed(t *testing.T) {
	d := newFakeDeps()
	d.canon[`d:\link`] = `D:\real\target`
	pl, _ := newTestPlanner(d)
	p, _ := pl.Plan([]Request{{Path: `D:\link`, Size: 5, Class: clsSafeDir}}, ModePermanent, nil)
	if len(p.Items) != 1 || p.Items[0].Path != `D:\real\target` {
		t.Fatalf("items = %+v", p.Items)
	}
	if !hasWarn(p.Items[0], WarnCanonicalDiffers) {
		t.Fatalf("missing canonical_differs: %+v", p.Items[0].Warnings)
	}
	// Same path in other case/shape: no warning.
	d.canon[`d:\real\target2\`] = `D:\Real\Target2`
	p, _ = pl.Plan([]Request{{Path: `d:\real\target2\`, Size: 5, Class: clsSafeDir}}, ModePermanent, nil)
	if len(p.Items) != 1 || hasWarn(p.Items[0], WarnCanonicalDiffers) {
		t.Fatalf("items = %+v", p.Items)
	}
}

func hasWarn(it PlanItem, w string) bool {
	for _, x := range it.Warnings {
		if x == w {
			return true
		}
	}
	return false
}

func TestStaleItemRefused(t *testing.T) {
	d := newFakeDeps()
	d.exist(`C:\a\node_modules`, `C:\c\node_modules`)
	pl, _ := newTestPlanner(d)
	reqs := []Request{
		{Path: `C:\a\node_modules`, Size: 10, Class: clsSafeDir},
		{Path: `C:\b\node_modules`, Size: 20, Class: clsSafeDir}, // vanished
		{Path: `C:\c\node_modules`, Size: 30, Class: clsSafeDir},
	}
	p, err := pl.Plan(reqs, ModeRecycle, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := refusalReason(p, `C:\b\node_modules`); got != ReasonNotFound {
		t.Fatalf("reason = %q", got)
	}
	if !accepted(p, `C:\a\node_modules`) || !accepted(p, `C:\c\node_modules`) || len(p.Items) != 2 {
		t.Fatalf("items = %+v", p.Items)
	}
	if p.TotalBytes != 40 {
		t.Fatalf("total = %d", p.TotalBytes)
	}
	if p.Token == "" || p.ID == "" {
		t.Fatal("missing token/id")
	}
}

func TestGitInsideNodeModulesAllowed(t *testing.T) {
	d := newFakeDeps()
	d.exist(`C:\p\node_modules`, `C:\p\stuff`)
	d.git[`c:\p\node_modules`] = true
	d.git[`c:\p\stuff`] = true
	pl, _ := newTestPlanner(d)
	p, err := pl.Plan([]Request{
		{Path: `C:\p\node_modules`, Size: 1, Class: clsSafeDir},
		{Path: `C:\p\stuff`, Size: 1, Class: clsUnknown},
	}, ModeRecycle, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !accepted(p, `C:\p\node_modules`) {
		t.Fatalf("node_modules refused: %+v", p.Refused)
	}
	if got := refusalReason(p, `C:\p\stuff`); got != ReasonContainsGit {
		t.Fatalf("reason = %q", got)
	}
	if d.calls[`c:\p\node_modules`] != 0 {
		t.Fatal("ContainsGit should not be walked for delete_dir items")
	}
}

func TestUnknownActionBecomesDeleteDir(t *testing.T) {
	d := newFakeDeps()
	d.exist(`C:\p\big`)
	pl, _ := newTestPlanner(d)
	p, _ := pl.Plan([]Request{{Path: `C:\p\big`, Size: 1, Class: clsUnknown}}, ModeRecycle, nil)
	if len(p.Items) != 1 || p.Items[0].Class.Action != rules.ActionDeleteDir {
		t.Fatalf("items = %+v", p.Items)
	}
	if p.Items[0].Class.Source != rules.SourceUnknown {
		t.Fatal("source must be preserved")
	}
}

func TestRecycleCapacityCumulative(t *testing.T) {
	d := newFakeDeps()
	d.exist(`C:\a`, `C:\b`)
	d.bins[`C:\`] = fakeBin{remaining: 100, has: true}
	reqs := []Request{{Path: `C:\a`, Size: 60, Class: clsSafeDir}, {Path: `C:\b`, Size: 60, Class: clsSafeDir}}
	pl, _ := newTestPlanner(d)
	p, _ := pl.Plan(reqs, ModeRecycle, nil)
	if !accepted(p, `C:\a`) {
		t.Fatalf("first refused: %+v", p.Refused)
	}
	if got := refusalReason(p, `C:\b`); got != ReasonExceedsRecycleBin {
		t.Fatalf("reason = %q", got)
	}
	p, _ = pl.Plan(reqs, ModePermanent, nil)
	if len(p.Items) != 2 {
		t.Fatalf("permanent: %+v / %+v", p.Items, p.Refused)
	}
}

func TestRecycleCapacityAfterDedupe(t *testing.T) {
	// The inner duplicate must not consume recycle-bin capacity.
	d := newFakeDeps()
	d.exist(`C:\a\inner`, `C:\a`, `C:\b`)
	d.bins[`C:\`] = fakeBin{remaining: 100, has: true}
	pl, _ := newTestPlanner(d)
	p, _ := pl.Plan([]Request{
		{Path: `C:\a\inner`, Size: 50, Class: clsSafeDir},
		{Path: `C:\a`, Size: 60, Class: clsSafeDir},
		{Path: `C:\b`, Size: 40, Class: clsSafeDir},
	}, ModeRecycle, nil)
	if got := refusalReason(p, `C:\a\inner`); got != ReasonDuplicate {
		t.Fatalf("inner reason = %q", got)
	}
	if !accepted(p, `C:\a`) || !accepted(p, `C:\b`) || len(p.Items) != 2 {
		t.Fatalf("items = %+v refused = %+v", p.Items, p.Refused)
	}
	if p.Items[0].Path != `C:\a` {
		t.Fatalf("order = %+v", p.Items)
	}
}

func TestDuplicates(t *testing.T) {
	d := newFakeDeps()
	d.exist(`C:\a`, `C:\a\b`, `C:\ab`)
	d.canon[`c:\link-to-a`] = `C:\a`
	pl, _ := newTestPlanner(d)
	p, _ := pl.Plan([]Request{
		{Path: `C:\a`, Size: 1, Class: clsSafeDir},
		{Path: `C:\a\b`, Size: 1, Class: clsSafeDir},
		{Path: `C:\link-to-a`, Size: 1, Class: clsSafeDir},
		{Path: `C:\ab`, Size: 1, Class: clsSafeDir},
	}, ModePermanent, nil)
	if refusalReason(p, `C:\a\b`) != ReasonDuplicate || refusalReason(p, `C:\link-to-a`) != ReasonDuplicate {
		t.Fatalf("refused = %+v", p.Refused)
	}
	if !accepted(p, `C:\a`) || !accepted(p, `C:\ab`) || len(p.Items) != 2 {
		t.Fatalf("items = %+v", p.Items)
	}
}

func TestManyFilesWarning(t *testing.T) {
	d := newFakeDeps()
	d.exist(`C:\many`, `C:\few`)
	pl, _ := newTestPlanner(d)
	count := func(p string) uint32 {
		if strings.EqualFold(p, `C:\many`) {
			return 25000
		}
		return 19999
	}
	reqs := []Request{{Path: `C:\many`, Size: 1, Class: clsSafeDir}, {Path: `C:\few`, Size: 1, Class: clsSafeDir}}
	p, _ := pl.Plan(reqs, ModeRecycle, count)
	if len(p.Items) != 2 || !hasWarn(p.Items[0], WarnManyFiles) || hasWarn(p.Items[1], WarnManyFiles) {
		t.Fatalf("items = %+v", p.Items)
	}
	p, _ = pl.Plan(reqs, ModePermanent, count)
	if hasWarn(p.Items[0], WarnManyFiles) {
		t.Fatal("many_files only applies to recycle mode")
	}
}

func TestInvalidMode(t *testing.T) {
	pl, _ := newTestPlanner(newFakeDeps())
	if _, err := pl.Plan(nil, Mode("shred"), nil); err == nil {
		t.Fatal("expected error")
	}
}

func onePlan(t *testing.T, pl *Planner, d *fakeDeps, path string) Plan {
	t.Helper()
	d.exist(path)
	p, err := pl.Plan([]Request{{Path: path, Size: 7, Class: clsSafeDir}}, ModeRecycle, nil)
	if err != nil || p.Token == "" || len(p.Items) != 1 {
		t.Fatalf("plan: %+v %v", p, err)
	}
	return p
}

func TestTokenFormat(t *testing.T) {
	d := newFakeDeps()
	pl, c := newTestPlanner(d)
	p := onePlan(t, pl, d, `C:\x`)
	if len(p.Token) != 64 || len(p.ID) != 32 {
		t.Fatalf("token %q id %q", p.Token, p.ID)
	}
	if !p.ExpiresAt.Equal(c.t.Add(5 * time.Minute)) {
		t.Fatalf("expires %v", p.ExpiresAt)
	}
	q := onePlan(t, pl, d, `C:\x`)
	if q.Token == p.Token || q.ID == p.ID {
		t.Fatal("token/id reused")
	}
}

func TestTokenSingleUse(t *testing.T) {
	d := newFakeDeps()
	pl, _ := newTestPlanner(d)
	p := onePlan(t, pl, d, `C:\x`)
	r, err := pl.Redeem(p.ID, p.Token)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items) != 1 || r.Items[0].Path != `C:\x` || r.Mode != ModeRecycle {
		t.Fatalf("redeemed = %+v", r)
	}
	if _, err := pl.Redeem(p.ID, p.Token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("second redeem err = %v", err)
	}
}

func TestTokenExpires(t *testing.T) {
	d := newFakeDeps()
	pl, c := newTestPlanner(d)
	p := onePlan(t, pl, d, `C:\x`)
	c.t = c.t.Add(5*time.Minute + time.Second)
	if _, err := pl.Redeem(p.ID, p.Token); !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v", err)
	}
	if _, err := pl.Redeem(p.ID, p.Token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("after expiry err = %v", err)
	}
}

func TestTokenExpiresAcrossPurge(t *testing.T) {
	// A later Plan call purges expired plans; Redeem still reports expiry.
	d := newFakeDeps()
	pl, c := newTestPlanner(d)
	p := onePlan(t, pl, d, `C:\x`)
	c.t = c.t.Add(6 * time.Minute)
	onePlan(t, pl, d, `C:\y`)
	if _, err := pl.Redeem(p.ID, p.Token); !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v", err)
	}
}

func TestTokenWrongPlan(t *testing.T) {
	d := newFakeDeps()
	pl, _ := newTestPlanner(d)
	a := onePlan(t, pl, d, `C:\a`)
	b := onePlan(t, pl, d, `C:\b`)
	if _, err := pl.Redeem(b.ID, a.Token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v", err)
	}
	// Single attempt: plan B is gone even with its own token now.
	if _, err := pl.Redeem(b.ID, b.Token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v", err)
	}
	// Plan A is unaffected.
	if _, err := pl.Redeem(a.ID, a.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := pl.Redeem("nope", ""); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v", err)
	}
}

func TestPlanIsCopied(t *testing.T) {
	d := newFakeDeps()
	pl, _ := newTestPlanner(d)
	p := onePlan(t, pl, d, `C:\x`)
	p.Items[0].Path = `C:\Windows`
	p.Items = append(p.Items, PlanItem{Path: `C:\Users`})
	r, err := pl.Redeem(p.ID, p.Token)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items) != 1 || r.Items[0].Path != `C:\x` {
		t.Fatalf("stored plan mutated: %+v", r.Items)
	}
}

func TestEmptyPlanHasNoToken(t *testing.T) {
	d := newFakeDeps()
	pl, _ := newTestPlanner(d)
	p, err := pl.Plan([]Request{{Path: `C:\gone`, Class: clsSafeDir}}, ModeRecycle, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Token != "" || len(p.Items) != 0 || len(p.Refused) != 1 {
		t.Fatalf("plan = %+v", p)
	}
	if _, err := pl.Redeem(p.ID, p.Token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v", err)
	}
}

func TestJunctionNamedNodeModulesIntoRepo(t *testing.T) {
	d := newFakeDeps()
	d.canon[`c:\p\node_modules`] = `C:\src\repo`
	d.git[`c:\src\repo`] = true
	pl, _ := newTestPlanner(d)
	p, _ := pl.Plan([]Request{{Path: `C:\p\node_modules`, Size: 1, Class: clsSafeDir}}, ModePermanent, nil)
	if got := refusalReason(p, `C:\p\node_modules`); got != ReasonContainsGit {
		t.Fatalf("reason = %q (items %+v)", got, p.Items)
	}
	// Same canonical form (case/shape only): the delete_dir exception holds.
	d.canon[`c:/q/node_modules/`] = `C:\Q\Node_Modules`
	d.git[`c:\q\node_modules`] = true
	p, _ = pl.Plan([]Request{{Path: `c:/q/node_modules/`, Size: 1, Class: clsSafeDir}}, ModePermanent, nil)
	if len(p.Items) != 1 {
		t.Fatalf("refused = %+v", p.Refused)
	}
}

func TestCapacityOverflow(t *testing.T) {
	d := newFakeDeps()
	d.exist(`C:\a`, `C:\b`)
	d.bins[`C:\`] = fakeBin{remaining: 100, has: true}
	pl, _ := newTestPlanner(d)
	p, _ := pl.Plan([]Request{
		{Path: `C:\a`, Size: 60, Class: clsSafeDir},
		{Path: `C:\b`, Size: math.MaxInt64, Class: clsSafeDir},
	}, ModeRecycle, nil)
	if got := refusalReason(p, `C:\b`); got != ReasonExceedsRecycleBin {
		t.Fatalf("reason = %q (items %+v)", got, p.Items)
	}
	if len(p.Items) != 1 || p.TotalBytes != 60 {
		t.Fatalf("items = %+v", p.Items)
	}
}

func TestInvalidSize(t *testing.T) {
	d := newFakeDeps()
	d.exist(`C:\a`)
	pl, _ := newTestPlanner(d)
	p, _ := pl.Plan([]Request{{Path: `C:\a`, Size: -1, Class: clsSafeDir}}, ModePermanent, nil)
	if got := refusalReason(p, `C:\a`); got != ReasonInvalidSize {
		t.Fatalf("reason = %q", got)
	}
}

func TestInvalidAction(t *testing.T) {
	d := newFakeDeps()
	d.exist(`C:\a`)
	pl, _ := newTestPlanner(d)
	cls := rules.Classification{Verdict: rules.Safe, Action: rules.Action("shred"), Source: rules.SourceAI}
	p, _ := pl.Plan([]Request{{Path: `C:\a`, Size: 1, Class: cls}}, ModePermanent, nil)
	if got := refusalReason(p, `C:\a`); got != ReasonInvalidAction {
		t.Fatalf("reason = %q", got)
	}
	// Checked after keep/manual, before not_found.
	cls.Verdict = rules.Keep
	p, _ = pl.Plan([]Request{{Path: `C:\a`, Size: 1, Class: cls}}, ModePermanent, nil)
	if got := refusalReason(p, `C:\a`); got != ReasonKeep {
		t.Fatalf("reason = %q", got)
	}
	cls.Verdict = rules.Safe
	p, _ = pl.Plan([]Request{{Path: `C:\gone`, Size: 1, Class: cls}}, ModePermanent, nil)
	if got := refusalReason(p, `C:\gone`); got != ReasonInvalidAction {
		t.Fatalf("reason = %q", got)
	}
}

func TestRecheck(t *testing.T) {
	d := newFakeDeps()
	pl, _ := newTestPlanner(d)
	dir := PlanItem{Path: `D:\proj\node_modules`, Size: 1, Class: clsSafeDir, RuleAction: rules.ActionDeleteDir}
	contents := PlanItem{Path: `D:\cache`, Size: 1, Class: clsSafeContents}

	// Missing.
	if got := pl.Recheck(dir); got != ReasonNotFound {
		t.Fatalf("missing: %q", got)
	}
	d.exist(dir.Path, contents.Path)
	if got := pl.Recheck(dir); got != "" {
		t.Fatalf("ok: %q", got)
	}
	// .git below a delete_dir item is allowed (same rule as Plan).
	d.git[strings.ToLower(dir.Path)] = true
	if got := pl.Recheck(dir); got != "" {
		t.Fatalf("delete_dir with git: %q", got)
	}
	d.git[strings.ToLower(contents.Path)] = true
	if got := pl.Recheck(contents); got != ReasonContainsGit {
		t.Fatalf("delete_contents with git: %q", got)
	}
	// Became a junction elsewhere.
	d.canon[strings.ToLower(dir.Path)] = `D:\repo`
	if got := pl.Recheck(dir); got != ReasonChanged {
		t.Fatalf("changed: %q", got)
	}
	// Same path, other case/separators: still the same item.
	d.canon[strings.ToLower(dir.Path)] = `d:/PROJ/node_modules/`
	if got := pl.Recheck(dir); got != "" {
		t.Fatalf("case differs: %q", got)
	}
	// Protected / contains protected.
	prot := PlanItem{Path: `C:\Windows\System32`, Size: 1, Class: clsSafeDir}
	d.exist(prot.Path)
	if got := pl.Recheck(prot); got != ReasonProtected {
		t.Fatalf("protected: %q", got)
	}
	users := PlanItem{Path: `C:\Users`, Size: 1, Class: clsSafeDir}
	d.exist(users.Path)
	if got := pl.Recheck(users); got != ReasonContainsProtected {
		t.Fatalf("contains protected: %q", got)
	}
}

// An unclassified item planned while clean must still be refused by Recheck
// once a .git appears: the delete_dir rewrite must not grant the exemption.
func TestRecheckUnknownItemGainsGit(t *testing.T) {
	d := newFakeDeps()
	d.exist(`C:\p\stuff`)
	pl, _ := newTestPlanner(d)
	p, _ := pl.Plan([]Request{{Path: `C:\p\stuff`, Size: 1, Class: clsUnknown}}, ModePermanent, nil)
	if len(p.Items) != 1 {
		t.Fatalf("refused = %+v", p.Refused)
	}
	d.git[`c:\p\stuff`] = true
	if got := pl.Recheck(p.Items[0]); got != ReasonContainsGit {
		t.Fatalf("Recheck = %q, want contains_git", got)
	}
}

// AI-sourced items never get the delete_dir .git exemption, in Plan or Recheck.
func TestAISourcedNoGitExemption(t *testing.T) {
	d := newFakeDeps()
	d.exist(`C:\p\ai`, `C:\p\ai2`)
	d.git[`c:\p\ai`] = true
	pl, _ := newTestPlanner(d)
	ai := rules.Classification{Verdict: rules.Safe, Source: rules.SourceAI}
	aiDir := rules.Classification{Verdict: rules.Safe, Action: rules.ActionDeleteDir, Source: rules.SourceAI}
	p, _ := pl.Plan([]Request{{Path: `C:\p\ai`, Size: 1, Class: aiDir}}, ModePermanent, nil)
	if got := refusalReason(p, `C:\p\ai`); got != ReasonContainsGit {
		t.Fatalf("plan ai delete_dir: %q", got)
	}
	p, _ = pl.Plan([]Request{{Path: `C:\p\ai2`, Size: 1, Class: ai}}, ModePermanent, nil)
	if len(p.Items) != 1 {
		t.Fatalf("refused = %+v", p.Refused)
	}
	d.git[`c:\p\ai2`] = true
	if got := pl.Recheck(p.Items[0]); got != ReasonContainsGit {
		t.Fatalf("Recheck ai = %q", got)
	}
	// A hand-built item claiming delete_dir without the rule origin is refused too.
	if got := pl.Recheck(PlanItem{Path: `C:\p\ai`, Size: 1, Class: aiDir}); got != ReasonContainsGit {
		t.Fatalf("Recheck ai delete_dir = %q", got)
	}
}
