package app

import (
	"context"
	"fmt"
	"testing"

	"github.com/engviniciusbs/nodegarbage/internal/scanner"
	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

// projectsScanner has n projects (C:\projN, each with a package.json per the
// test classifier) whose node_modules grow with N, plus a nested one.
type projectsScanner struct{ n int }

func (p projectsScanner) Scan(_ context.Context, root string, b *tree.Builder, _ *scanner.Progress) error {
	r := b.AddDir(tree.NoNode, root, 0)
	for i := 1; i <= p.n; i++ {
		proj := b.AddDir(r, fmt.Sprintf("proj%d", i), 0)
		nm := b.AddDir(proj, "node_modules", 0)
		b.SetOwnStats(nm, int64(i*100), 1, 100)
		inner := b.AddDir(nm, "node_modules", 0)
		b.SetOwnStats(inner, 10, 1, 100)
	}
	return nil
}

func scannedWith(t *testing.T, sc scanner.Scanner) *App {
	t.Helper()
	a, _ := newTestApp(t, sc, nil)
	if err := a.StartScan([]string{`C:\`}); err != nil {
		t.Fatal(err)
	}
	a.waitScan()
	return a
}

func TestListFilteredByCategoryTopLevelOnly(t *testing.T) {
	a := scanned(t)
	f, err := a.ListFiltered(`C:\`, FilterDTO{Category: "build-artifact"})
	if err != nil {
		t.Fatal(err)
	}
	// the inner node_modules inherits the class but sits under a match
	if len(f.Items) != 1 || f.Count != 1 || f.Truncated {
		t.Fatalf("filtered=%+v", f)
	}
	it := f.Items[0]
	if it.Path != `C:\proj\node_modules` || it.Location != `C:\proj` || it.Size != 1500 || it.RuleID != "node.node_modules" {
		t.Fatalf("item=%+v", it)
	}
	if f.TotalBytes != 1500 {
		t.Fatalf("total=%d", f.TotalBytes)
	}
}

func TestListFilteredByRuleIDSortedWithLocation(t *testing.T) {
	a := scannedWith(t, projectsScanner{n: 3})
	f, err := a.ListFiltered(`C:\`, FilterDTO{RuleID: "node.node_modules"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Count != 3 || len(f.Items) != 3 || f.TotalBytes != 310+210+110 {
		t.Fatalf("filtered=%+v", f)
	}
	for i, want := range []string{`C:\proj3`, `C:\proj2`, `C:\proj1`} {
		if f.Items[i].Location != want || f.Items[i].Name != "node_modules" {
			t.Fatalf("items[%d]=%+v want location %s", i, f.Items[i], want)
		}
	}
	// normal listings leave Location empty
	root, _ := a.GetChildren(`C:\`, -1)
	if root[0].Location != "" {
		t.Fatalf("root[0].Location=%q", root[0].Location)
	}
}

func TestListFilteredCapKeepsTotals(t *testing.T) {
	old := maxFiltered
	maxFiltered = 2
	t.Cleanup(func() { maxFiltered = old })
	a := scannedWith(t, projectsScanner{n: 3})
	f, err := a.ListFiltered(`C:\`, FilterDTO{RuleID: "node.node_modules"})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Items) != 2 || !f.Truncated || f.Count != 3 || f.TotalBytes != 630 {
		t.Fatalf("filtered=%+v", f)
	}
	if f.Items[0].Size != 310 || f.Items[1].Size != 210 {
		t.Fatalf("cap must keep the largest: %+v", f.Items)
	}
}

func TestListFilteredAICategory(t *testing.T) {
	a, _, _, _, p := newAIApp(t)
	if err := a.RunAI(p.ID, false); err != nil {
		t.Fatal(err)
	}
	a.waitAI()
	f, err := a.ListFiltered(`C:\`, FilterDTO{Category: "ai"})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, it := range f.Items {
		if it.Path == `C:\other` && it.Source == "ai" {
			found = true
		}
	}
	if !found {
		t.Fatalf("filtered=%+v", f)
	}
}

func TestListFilteredErrors(t *testing.T) {
	a := scanned(t)
	for _, f := range []FilterDTO{{}, {Category: "temp", RuleID: "x"}} {
		if _, err := a.ListFiltered(`C:\`, f); err == nil {
			t.Fatalf("filter %+v: expected error", f)
		}
	}
	_, err := a.ListFiltered(`Z:\`, FilterDTO{Category: "temp"})
	_, want := a.GetChildren(`Z:\`, -1)
	if err == nil || err.Error() != want.Error() {
		t.Fatalf("err=%v want %v", err, want)
	}
	f, err := a.ListFiltered(`C:\`, FilterDTO{Category: "temp"})
	if err != nil || f.Count != 0 || f.Items == nil {
		t.Fatalf("empty filter=%+v err=%v", f, err)
	}
}
