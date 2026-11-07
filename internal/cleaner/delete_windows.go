//go:build windows

package cleaner

import (
	"errors"
	"fmt"
	"io/fs"
	"unsafe"

	"golang.org/x/sys/windows"
)

// OSDeleter returns the real Windows Deleter.
func OSDeleter() Deleter { return osDeleter{} }

type osDeleter struct{}

// Attributes SetFileAttributes accepts; used when clearing read-only.
const settableAttrs = windows.FILE_ATTRIBUTE_ARCHIVE | windows.FILE_ATTRIBUTE_HIDDEN |
	windows.FILE_ATTRIBUTE_NOT_CONTENT_INDEXED | windows.FILE_ATTRIBUTE_OFFLINE |
	windows.FILE_ATTRIBUTE_SYSTEM | windows.FILE_ATTRIBUTE_TEMPORARY

func (osDeleter) RemoveAll(path string) (int64, []string, error) { return removeTree(path, true) }

func (osDeleter) RemoveContents(path string) (int64, []string, error) {
	return removeTree(path, false)
}

// remover accumulates the outcome of one removeTree call.
type remover struct {
	freed   int64
	removed int // entries (files, links, directories) actually removed
	skipped []string
}

// removeTree deletes below path (and path itself if removeRoot), through \\?\
// paths, never following reparse points: a junction, symbolic link or mount
// point is removed as a link (RemoveDirectory/DeleteFile on the link), never
// entered. If the root is a reparse point, RemoveAll removes just the link
// and RemoveContents refuses with ErrReparseRoot.
func removeTree(path string, removeRoot bool) (int64, []string, error) {
	ext, err := absExtended(path)
	if err != nil {
		return 0, nil, &fs.PathError{Op: "remove", Path: path, Err: err}
	}
	p, err := windows.UTF16PtrFromString(ext)
	if err != nil {
		return 0, nil, &fs.PathError{Op: "remove", Path: path, Err: err}
	}
	var data windows.Win32FileAttributeData
	if err := windows.GetFileAttributesEx(p, windows.GetFileExInfoStandard, (*byte)(unsafe.Pointer(&data))); err != nil {
		return 0, nil, &fs.PathError{Op: "remove", Path: path, Err: err}
	}
	attrs := data.FileAttributes
	isDir := attrs&windows.FILE_ATTRIBUTE_DIRECTORY != 0
	isLink := attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0

	r := &remover{}
	switch {
	case isLink && !removeRoot:
		return 0, nil, fmt.Errorf("%w: %s", ErrReparseRoot, path)
	case !isDir && !removeRoot:
		return 0, nil, &fs.PathError{Op: "remove contents", Path: path, Err: windows.ERROR_DIRECTORY}
	case removeRoot:
		size := int64(data.FileSizeHigh)<<32 | int64(data.FileSizeLow)
		r.removeEntry(ext, attrs, size)
	default:
		r.removeContents(ext)
	}
	if r.removed == 0 && len(r.skipped) > 0 {
		return 0, r.skipped, fmt.Errorf("%w: %s", ErrNothingRemoved, path)
	}
	return r.freed, r.skipped, nil
}

func (r *remover) skip(ext string) { r.skipped = append(r.skipped, stripExtended(ext)) }

func isNotExist(err error) bool {
	return errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND)
}

// removeEntry removes one entry, given its attributes from the directory
// listing (which describe the entry itself, not a link's target).
func (r *remover) removeEntry(ext string, attrs uint32, size int64) {
	switch {
	case attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0:
		r.removeLink(ext, attrs)
	case attrs&windows.FILE_ATTRIBUTE_DIRECTORY != 0:
		// Re-read right before descending: a directory replaced by a
		// junction since it was listed must be removed as a link.
		if cur, err := currentAttrs(ext); err == nil && cur&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			r.removeLink(ext, cur)
			return
		}
		before := len(r.skipped)
		r.removeContents(ext)
		r.removeDir(ext, attrs, len(r.skipped) > before)
	default:
		r.removeFile(ext, attrs, size)
	}
}

// currentAttrs returns the attributes of the entry itself (not following
// reparse points).
func currentAttrs(ext string) (uint32, error) {
	p, err := windows.UTF16PtrFromString(ext)
	if err != nil {
		return 0, err
	}
	return windows.GetFileAttributes(p)
}

// removeLink deletes a reparse point itself. Its attributes are not touched:
// SetFileAttributes could act on the link's target.
func (r *remover) removeLink(ext string, attrs uint32) {
	p, err := windows.UTF16PtrFromString(ext)
	if err == nil {
		if attrs&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
			err = windows.RemoveDirectory(p)
		} else {
			err = windows.DeleteFile(p)
		}
	}
	switch {
	case err == nil:
		r.removed++
	case isNotExist(err):
	default:
		r.skip(ext)
	}
}

// removeFile deletes a regular file, clearing read-only first (restored if
// the delete fails). Sharing violations, denied access and other failures
// skip the file.
func (r *remover) removeFile(ext string, attrs uint32, size int64) {
	p, err := windows.UTF16PtrFromString(ext)
	if err != nil {
		r.skip(ext)
		return
	}
	cleared := clearReadOnly(p, attrs)
	err = windows.DeleteFile(p)
	switch {
	case err == nil:
		r.freed += size
		r.removed++
		return
	case isNotExist(err):
		return
	}
	if cleared {
		_ = windows.SetFileAttributes(p, attrs&(settableAttrs|windows.FILE_ATTRIBUTE_READONLY))
	}
	r.skip(ext)
}

// clearReadOnly clears FILE_ATTRIBUTE_READONLY of a non-link entry and
// reports whether it did.
func clearReadOnly(p *uint16, attrs uint32) bool {
	if attrs&windows.FILE_ATTRIBUTE_READONLY == 0 {
		return false
	}
	na := attrs & settableAttrs
	if na == 0 {
		na = windows.FILE_ATTRIBUTE_NORMAL
	}
	return windows.SetFileAttributes(p, na) == nil
}

// removeDir removes an (expected empty) directory. When it still holds
// skipped entries the failure is already accounted for by them.
func (r *remover) removeDir(ext string, attrs uint32, childSkipped bool) {
	p, err := windows.UTF16PtrFromString(ext)
	if err != nil {
		r.skip(ext)
		return
	}
	clearReadOnly(p, attrs)
	err = windows.RemoveDirectory(p)
	switch {
	case err == nil:
		r.removed++
	case isNotExist(err):
	case childSkipped && errors.Is(err, windows.ERROR_DIR_NOT_EMPTY):
	default:
		r.skip(ext)
	}
}

type dirEntry struct {
	name  string
	attrs uint32
	size  int64
}

// removeContents lists dir completely, then removes each entry. A directory
// that cannot be listed is skipped.
func (r *remover) removeContents(dir string) {
	ents, err := listDir(dir)
	if err != nil {
		if !isNotExist(err) {
			r.skip(dir)
		}
		return
	}
	for _, e := range ents {
		r.removeEntry(dir+`\`+e.name, e.attrs, e.size)
	}
}

func listDir(dir string) ([]dirEntry, error) {
	pat, err := windows.UTF16PtrFromString(dir + `\*`)
	if err != nil {
		return nil, err
	}
	var fd windows.Win32finddata
	h, err := windows.FindFirstFile(pat, &fd)
	if err != nil {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			return nil, nil // no entries at all
		}
		return nil, err
	}
	defer windows.FindClose(h)
	var out []dirEntry
	for {
		name := windows.UTF16ToString(fd.FileName[:])
		if name != "." && name != ".." {
			out = append(out, dirEntry{
				name:  name,
				attrs: fd.FileAttributes,
				size:  int64(fd.FileSizeHigh)<<32 | int64(fd.FileSizeLow),
			})
		}
		if err := windows.FindNextFile(h, &fd); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				return out, nil
			}
			return nil, err
		}
	}
}
