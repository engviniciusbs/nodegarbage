//go:build windows

package scanner

import (
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

type osFS struct{}

// OSFS returns the real Windows file system. It adds the \\?\ prefix only
// when calling the OS; callers always use plain paths.
func OSFS() FS { return osFS{} }

// extendedPath converts a plain absolute path to its \\?\ form.
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

func (osFS) ReadDir(path string) ([]DirEntry, error) {
	f, err := os.Open(extendedPath(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ents, err := f.ReadDir(-1) // unsorted: cheaper than os.ReadDir
	if err != nil && len(ents) == 0 {
		return nil, err
	}
	out := make([]DirEntry, 0, len(ents))
	for _, e := range ents {
		info, ierr := e.Info() // cached from FindNextFile on Windows
		if ierr != nil {
			continue // vanished between listing and stat
		}
		d := DirEntry{Name: e.Name(), ModTime: info.ModTime().Unix()}
		if a, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
			d.Attrs = a.FileAttributes
		}
		// Derive IsDir from attributes: Go reports junctions as irregular,
		// not as directories.
		d.IsDir = d.Attrs&windows.FILE_ATTRIBUTE_DIRECTORY != 0 || (d.Attrs == 0 && e.IsDir())
		if !d.IsDir {
			d.Size = info.Size()
		}
		out = append(out, d)
	}
	return out, nil
}
