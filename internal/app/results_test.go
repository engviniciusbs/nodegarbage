package app

import (
	"testing"
)

func scanned(t *testing.T) *App {
	t.Helper()
	a, _ := newTestApp(t, fakeScanner{}, nil)
	if err := a.StartScan([]string{`C:\`}); err != nil {
		t.Fatal(err)
	}
	a.waitScan()
	return a
}

func findNode(t *testing.T, nodes []NodeDTO, name string) NodeDTO {
	t.Helper()
	for _, n := range nodes {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("node %q not found", name)
	return NodeDTO{}
}

func checkChildren(t *testing.T, a *App) {
	t.Helper()
	root, err := a.GetChildren(`C:\`, -1)
	if err != nil || len(root) != 2 || root[0].Name != "proj" {
		t.Fatalf("root children=%v err=%v", root, err)
	}
	kids, err := a.GetChildren(`C:\`, root[0].ID)
	if err != nil || len(kids) != 2 {
		t.Fatalf("kids=%v err=%v", kids, err)
	}
	if kids[0].Size < kids[1].Size {
		t.Fatal("not sorted desc")
	}
	nm := findNode(t, kids, "node_modules")
	if nm.Verdict != "safe" || nm.Explain == "" || nm.Regenerable == "" || !nm.HasChildren {
		t.Fatalf("nm=%+v", nm)
	}
	if src := findNode(t, kids, "src"); src.Verdict != "unknown" {
		t.Fatalf("src verdict=%q", src.Verdict)
	}
}

func TestGetChildrenSortedWithClassification(t *testing.T) {
	checkChildren(t, scanned(t))
}

func TestSummaryNoDoubleCount(t *testing.T) {
	a := scanned(t)
	s, err := a.GetSummary(`C:\`)
	if err != nil {
		t.Fatal(err)
	}
	if s.SafeBytes != 1500 {
		t.Fatalf("safe=%d want 1500", s.SafeBytes)
	}
	var found bool
	for _, c := range s.ByCategory {
		if c.Category == "build-artifact" {
			found = true
			if c.Count != 1 || c.Bytes != 1500 {
				t.Fatalf("cats=%+v", s.ByCategory)
			}
		}
	}
	if !found {
		t.Fatalf("cats=%+v", s.ByCategory)
	}
}

func TestReloadFromSnapshotReclassifies(t *testing.T) {
	a := scanned(t)
	b, _ := newTestApp(t, fakeScanner{}, a.deps.Store)
	if _, err := b.GetChildren(`C:\`, -1); err == nil {
		t.Fatal("expected error before Startup")
	}
	b.Startup(nil)
	checkChildren(t, b)
	if _, err := b.GetChildren(`Z:\`, -1); err == nil {
		t.Fatal("expected volume not scanned")
	}
}

func TestExtensionsAndTopFilesEmpty(t *testing.T) {
	a := scanned(t)
	if _, err := a.GetExtensions(`C:\`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetTopFiles(`C:\`); err != nil {
		t.Fatal(err)
	}
}
