//go:build windows

package volumes

import (
	"os"
	"testing"
)

func TestListIncludesSystemDrive(t *testing.T) {
	vols, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := os.Getenv("SystemDrive") + `\`
	for _, v := range vols {
		if v.Root != want {
			continue
		}
		if v.Type != "fixed" {
			t.Errorf("Type = %q, want fixed", v.Type)
		}
		if v.Total == 0 {
			t.Error("Total = 0")
		}
		if v.Free > v.Total {
			t.Errorf("Free %d > Total %d", v.Free, v.Total)
		}
		if v.FS == "" {
			t.Error("FS empty")
		}
		if !v.HasRecycleBin {
			t.Error("HasRecycleBin should be true for fixed")
		}
		return
	}
	t.Fatalf("system drive %q not found in %+v", want, vols)
}
