//go:build windows

// Package volumes enumerates the logical drives of the machine.
package volumes

import (
	"sort"

	"golang.org/x/sys/windows"
)

// DriveType is the kind of drive: "fixed", "removable", "network", "cdrom", "ram" or "unknown".
type DriveType string

// Volume describes one mounted logical drive.
type Volume struct {
	Root          string // "C:\"
	Label, FS     string
	Type          DriveType
	Total, Free   uint64
	HasRecycleBin bool // Type == "fixed"
}

func driveType(t uint32) DriveType {
	switch t {
	case windows.DRIVE_FIXED:
		return "fixed"
	case windows.DRIVE_REMOVABLE:
		return "removable"
	case windows.DRIVE_REMOTE:
		return "network"
	case windows.DRIVE_CDROM:
		return "cdrom"
	case windows.DRIVE_RAMDISK:
		return "ram"
	}
	return "unknown"
}

// List returns the ready drives sorted by Root. Drives without media or that
// cannot be queried are silently omitted.
func List() ([]Volume, error) {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, err
	}
	var out []Volume
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		rootPtr, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		var label, fs [windows.MAX_PATH + 1]uint16
		if err := windows.GetVolumeInformation(rootPtr, &label[0], uint32(len(label)),
			nil, nil, nil, &fs[0], uint32(len(fs))); err != nil {
			continue
		}
		var free, total, totalFree uint64
		if err := windows.GetDiskFreeSpaceEx(rootPtr, &free, &total, &totalFree); err != nil {
			continue
		}
		t := driveType(windows.GetDriveType(rootPtr))
		out = append(out, Volume{
			Root:          root,
			Label:         windows.UTF16ToString(label[:]),
			FS:            windows.UTF16ToString(fs[:]),
			Type:          t,
			Total:         total,
			Free:          free,
			HasRecycleBin: t == "fixed",
		})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Root < out[b].Root })
	return out, nil
}
