package app

import "testing"

func TestLocatePathChain(t *testing.T) {
	a := scanned(t)
	d, err := a.LocatePath(`c:/PROJ\node_modules\node_modules\`)
	if err != nil {
		t.Fatal(err)
	}
	if d.Volume != `C:\` {
		t.Fatalf("volume=%q", d.Volume)
	}
	if len(d.Chain) != 3 || d.Chain[0].Name != "proj" || d.Chain[1].Name != "node_modules" || d.Chain[2].Name != "node_modules" {
		t.Fatalf("chain=%+v", d.Chain)
	}
	// every chain entry must be loadable as a child of the previous one
	parent := int32(-1)
	for _, n := range d.Chain {
		kids, err := a.GetChildren(d.Volume, parent)
		if err != nil {
			t.Fatal(err)
		}
		_ = findNode(t, kids, n.Name)
		parent = n.ID
	}
}

func TestLocatePathTopLevelAndErrors(t *testing.T) {
	a := scanned(t)
	d, err := a.LocatePath(`C:\other`)
	if err != nil || len(d.Chain) != 1 || d.Chain[0].Name != "other" {
		t.Fatalf("d=%+v err=%v", d, err)
	}
	for _, p := range []string{`C:\nope`, `D:\x`, `C:\`, ``} {
		if _, err := a.LocatePath(p); err == nil || err.Error() != "item não encontrado na última varredura" {
			t.Fatalf("path %q err=%v", p, err)
		}
	}
}
