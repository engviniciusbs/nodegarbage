//go:build windows

package cleaner

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// OSDeps returns the real Windows implementation of Deps.
func OSDeps() Deps { return osDeps{} }

type osDeps struct{}

const (
	volumeNameDOS      = 0x0 // VOLUME_NAME_DOS
	fileNameNormalized = 0x0 // FILE_NAME_NORMALIZED
	bitBucketVolumeKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\BitBucket\Volume\`
)

var procSHQueryRecycleBinW = windows.NewLazySystemDLL("shell32.dll").NewProc("SHQueryRecycleBinW")

// extendedPath converts an absolute path to its \\?\ form.
func extendedPath(p string) string {
	switch {
	case strings.HasPrefix(p, `\\?\`):
		return p
	case strings.HasPrefix(p, `\\`):
		return `\\?\UNC\` + p[2:]
	default:
		return `\\?\` + p
	}
}

// stripExtended removes the \\?\ (or \\?\UNC\) prefix.
func stripExtended(p string) string {
	switch {
	case len(p) >= 8 && strings.EqualFold(p[:8], `\\?\UNC\`):
		return `\\` + p[8:]
	case strings.HasPrefix(p, `\\?\`):
		return p[4:]
	}
	return p
}

func absExtended(path string) (string, error) {
	if strings.HasPrefix(path, `\\?\`) {
		return path, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return extendedPath(abs), nil
}

// Canonicalize opens the item (following junctions and symlinks, without
// requesting any access) and returns GetFinalPathNameByHandle's DOS path.
func (osDeps) Canonicalize(path string) (string, error) {
	ext, err := absExtended(path)
	if err != nil {
		return "", &fs.PathError{Op: "canonicalize", Path: path, Err: err}
	}
	p, err := windows.UTF16PtrFromString(ext)
	if err != nil {
		return "", &fs.PathError{Op: "canonicalize", Path: path, Err: err}
	}
	h, err := windows.CreateFile(p, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		// windows.Errno matches fs.ErrNotExist for file/path not found.
		return "", &fs.PathError{Op: "canonicalize", Path: path, Err: err}
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH+1)
	for {
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), volumeNameDOS|fileNameNormalized)
		if err != nil {
			return "", &fs.PathError{Op: "canonicalize", Path: path, Err: err}
		}
		if int(n) < len(buf) {
			return stripExtended(windows.UTF16ToString(buf[:n])), nil
		}
		buf = make([]uint16, n+1)
	}
}

// ContainsGit walks path (not following junctions or symlinks) and stops at
// the first entry named .git (case-insensitive; a .git file, as used by
// submodules and worktrees, also counts). Any error other than a vanished
// entry makes it answer true: absence of .git could not be proven.
func (osDeps) ContainsGit(path string) bool {
	root, err := absExtended(path)
	if err != nil {
		return true
	}
	found := false
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			found = true
			return fs.SkipAll
		}
		if p == root {
			return nil
		}
		if strings.EqualFold(d.Name(), ".git") {
			found = true
			return fs.SkipAll
		}
		if d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 && d.IsDir() {
			return fs.SkipDir
		}
		return nil
	})
	return found
}

// RecycleBinRemaining: only fixed drives have a bin. A volume configured to
// delete immediately (NukeOnDelete) has no usable bin either. Capacity is
// MaxCapacity (MB) from the per-volume BitBucket key, else 5% of the volume;
// current usage comes from SHQueryRecycleBinW.
func (osDeps) RecycleBinRemaining(volumeRoot string) (int64, bool, error) {
	rootPtr, err := windows.UTF16PtrFromString(volumeRoot)
	if err != nil {
		return 0, false, err
	}
	if windows.GetDriveType(rootPtr) != windows.DRIVE_FIXED {
		return 0, false, nil
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(rootPtr, &free, &total, &totalFree); err != nil {
		return 0, true, err
	}
	capacity := int64(total / 20)
	if guid := volumeGUID(rootPtr); guid != "" {
		if k, err := registry.OpenKey(registry.CURRENT_USER, bitBucketVolumeKey+guid, registry.QUERY_VALUE); err == nil {
			defer k.Close()
			if nuke, _, err := k.GetIntegerValue("NukeOnDelete"); err == nil && nuke != 0 {
				return 0, false, nil
			}
			if mb, _, err := k.GetIntegerValue("MaxCapacity"); err == nil {
				capacity = int64(mb) << 20
			}
		}
	}
	used, err := recycleBinUsed(rootPtr)
	if err != nil {
		return 0, true, err
	}
	return max(capacity-used, 0), true, nil
}

// volumeGUID returns "{...}" for a mount point such as `C:\`, or "".
func volumeGUID(rootPtr *uint16) string {
	var buf [64]uint16
	if err := windows.GetVolumeNameForVolumeMountPoint(rootPtr, &buf[0], uint32(len(buf))); err != nil {
		return ""
	}
	s := windows.UTF16ToString(buf[:])
	i, j := strings.IndexByte(s, '{'), strings.IndexByte(s, '}')
	if i < 0 || j < i {
		return ""
	}
	return s[i : j+1]
}

// shQueryRBInfo is SHQUERYRBINFO (natural alignment on 64-bit; on 386 both
// the C header and Go use 4-byte alignment for the int64 fields).
type shQueryRBInfo struct {
	cbSize   uint32
	size     int64
	numItems int64
}

func recycleBinUsed(rootPtr *uint16) (int64, error) {
	if err := procSHQueryRecycleBinW.Find(); err != nil {
		return 0, err
	}
	info := shQueryRBInfo{cbSize: uint32(unsafe.Sizeof(shQueryRBInfo{}))}
	hr, _, _ := procSHQueryRecycleBinW.Call(uintptr(unsafe.Pointer(rootPtr)), uintptr(unsafe.Pointer(&info)))
	if hr != 0 {
		return 0, fmt.Errorf("SHQueryRecycleBinW: HRESULT 0x%08x", uint32(hr))
	}
	return info.size, nil
}

// VolumeOf returns the volume root of path, e.g. `C:\` or `\\server\share\`.
func (osDeps) VolumeOf(path string) string {
	v := filepath.VolumeName(stripExtended(path))
	if v == "" {
		return ""
	}
	if len(v) == 2 && v[1] == ':' {
		v = strings.ToUpper(v)
	}
	return v + `\`
}
