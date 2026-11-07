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

// afterListHook, when set (tests only), is called with the display path of
// each directory right after it was listed and before its children are
// processed.
var afterListHook func(dir string)

const (
	shareAll = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE
	// accessFull lets one handle be inspected, listed, deleted and (in the
	// fallback path) have its read-only attribute cleared.
	accessFull = windows.DELETE | windows.FILE_READ_ATTRIBUTES | windows.FILE_WRITE_ATTRIBUTES |
		windows.FILE_LIST_DIRECTORY | windows.SYNCHRONIZE
	// accessDelete is the retry when FILE_LIST_DIRECTORY (= FILE_READ_DATA)
	// conflicts with another opener's share mode or is denied by the ACL.
	accessDelete = windows.DELETE | windows.FILE_READ_ATTRIBUTES | windows.SYNCHRONIZE
)

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

func (r *remover) skip(display string) { r.skipped = append(r.skipped, display) }

// removeTree deletes below path (and path itself if removeRoot).
//
// The walk is handle-relative: the root is opened once (without following a
// reparse point at its last component); every child is opened with
// NtCreateFile relative to its parent's handle with FILE_OPEN_REPARSE_POINT,
// inspected through that handle, listed through that handle and deleted
// through that handle (FileDispositionInformationEx). No step re-resolves a
// path, so swapping a directory for a junction mid-walk cannot redirect it.
// A reparse point (junction, symbolic link, mount point) is deleted as the
// link itself and never entered or modified. If the root is a reparse point,
// RemoveAll removes just the link and RemoveContents refuses with
// ErrReparseRoot.
func removeTree(path string, removeRoot bool) (int64, []string, error) {
	ext, err := absExtended(path)
	if err != nil {
		return 0, nil, &fs.PathError{Op: "remove", Path: path, Err: err}
	}
	display := stripExtended(ext)
	h, canDelete, err := openRootHandle(ext, removeRoot)
	if err != nil {
		return 0, nil, &fs.PathError{Op: "remove", Path: path, Err: err}
	}
	defer windows.CloseHandle(h)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return 0, nil, &fs.PathError{Op: "remove", Path: path, Err: err}
	}
	attrs := info.FileAttributes

	r := &remover{}
	switch {
	case attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 && !removeRoot:
		return 0, nil, fmt.Errorf("%w: %s", ErrReparseRoot, path)
	case attrs&windows.FILE_ATTRIBUTE_DIRECTORY == 0 && !removeRoot:
		return 0, nil, &fs.PathError{Op: "remove contents", Path: path, Err: windows.ERROR_DIRECTORY}
	case removeRoot:
		r.process(h, display, &info, canDelete)
	default:
		r.contents(h, display)
	}
	if r.removed == 0 && len(r.skipped) > 0 {
		return 0, r.skipped, fmt.Errorf("%w: %s", ErrNothingRemoved, path)
	}
	return r.freed, r.skipped, nil
}

// openRootHandle opens the item root without following a reparse point at
// its last component. canDelete is false when DELETE access could not be had
// (the root is then kept and reported as skipped).
func openRootHandle(ext string, wantDelete bool) (h windows.Handle, canDelete bool, err error) {
	p, err := windows.UTF16PtrFromString(ext)
	if err != nil {
		return 0, false, err
	}
	tries := []uint32{accessFull, accessFull &^ windows.DELETE}
	if !wantDelete {
		tries = tries[1:]
	}
	for _, acc := range tries {
		h, err = windows.CreateFile(p, acc, shareAll, nil, windows.OPEN_EXISTING,
			windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if err == nil {
			return h, acc&windows.DELETE != 0, nil
		}
		if !retryable(err) {
			break
		}
	}
	return 0, false, err
}

func retryable(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}

// openRel opens name (a single component) relative to the directory handle
// parent, never following a reparse point.
func openRel(parent windows.Handle, name string) (windows.Handle, error) {
	uname, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, err
	}
	oa := windows.OBJECT_ATTRIBUTES{RootDirectory: parent, ObjectName: uname}
	oa.Length = uint32(unsafe.Sizeof(oa))
	for _, acc := range []uint32{accessFull, accessDelete} {
		var h windows.Handle
		var iosb windows.IO_STATUS_BLOCK
		err = windows.NtCreateFile(&h, acc, &oa, &iosb, nil, 0, shareAll, windows.FILE_OPEN,
			windows.FILE_OPEN_REPARSE_POINT|windows.FILE_OPEN_FOR_BACKUP_INTENT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
		if err == nil {
			return h, nil
		}
		err = ntErrno(err)
		if !retryable(err) {
			break
		}
	}
	return 0, err
}

// ntErrno converts an NTSTATUS error to the matching Win32 Errno.
func ntErrno(err error) error {
	var st windows.NTStatus
	if errors.As(err, &st) {
		return st.Errno()
	}
	return err
}

func isNotExist(err error) bool {
	return errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND)
}

// process handles an opened entry whose information is info.
func (r *remover) process(h windows.Handle, display string, info *windows.ByHandleFileInformation, canDelete bool) {
	attrs := info.FileAttributes
	switch {
	case attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0:
		r.remove(h, display, attrs, 0, false, canDelete) // the link itself
	case attrs&windows.FILE_ATTRIBUTE_DIRECTORY != 0:
		before := len(r.skipped)
		r.contents(h, display)
		r.remove(h, display, attrs, 0, len(r.skipped) > before, canDelete)
	default:
		r.remove(h, display, attrs, int64(info.FileSizeHigh)<<32|int64(info.FileSizeLow), false, canDelete)
	}
}

// contents lists the directory handle dh completely, then opens and
// processes each child relative to dh.
func (r *remover) contents(dh windows.Handle, display string) {
	names, err := listHandle(dh)
	if err != nil {
		r.skip(display)
		return
	}
	if afterListHook != nil {
		afterListHook(display)
	}
	for _, name := range names {
		child := display + `\` + name
		h, err := openRel(dh, name)
		if err != nil {
			if !isNotExist(err) {
				r.skip(child)
			}
			continue
		}
		var info windows.ByHandleFileInformation
		if err := windows.GetFileInformationByHandle(h, &info); err != nil {
			r.skip(child)
		} else {
			r.process(h, child, &info, true)
		}
		windows.CloseHandle(h)
	}
}

// remove deletes the entry behind h. A directory that still holds skipped
// entries is accounted for by them.
func (r *remover) remove(h windows.Handle, display string, attrs uint32, size int64, childSkipped, canDelete bool) {
	if !canDelete {
		if !childSkipped {
			r.skip(display)
		}
		return
	}
	err := deleteHandle(h, attrs)
	switch {
	case err == nil:
		r.removed++
		r.freed += size
	case isNotExist(err):
	case childSkipped && errors.Is(err, windows.ERROR_DIR_NOT_EMPTY):
	default:
		r.skip(display)
	}
}

// forceDispositionFallback (tests only) makes deleteHandle skip
// FileDispositionInformationEx, as on pre-1809 Windows or FAT volumes.
var forceDispositionFallback bool

// fileDispositionInfoEx is FILE_DISPOSITION_INFORMATION_EX.
type fileDispositionInfoEx struct{ Flags uint32 }

// fileDispositionInfo is FILE_DISPOSITION_INFO (BOOLEAN DeleteFile).
type fileDispositionInfo struct{ DeleteFile uint8 }

// fileBasicInfo is FILE_BASIC_INFO; zero times mean "unchanged".
type fileBasicInfo struct {
	CreationTime, LastAccessTime, LastWriteTime, ChangeTime int64
	FileAttributes                                          uint32
	_                                                       uint32
}

// deleteHandle marks the object behind h for deletion. It first uses POSIX
// semantics with FILE_DISPOSITION_IGNORE_READONLY_ATTRIBUTE (Windows 10
// 1809+, NTFS), which deletes read-only entries without changing any
// attribute. Where that is unsupported it falls back to FileDispositionInfo,
// clearing read-only through the same handle first; a reparse point's
// attributes are never changed.
func deleteHandle(h windows.Handle, attrs uint32) error {
	var iosb windows.IO_STATUS_BLOCK
	ex := fileDispositionInfoEx{Flags: windows.FILE_DISPOSITION_DELETE | windows.FILE_DISPOSITION_POSIX_SEMANTICS |
		windows.FILE_DISPOSITION_IGNORE_READONLY_ATTRIBUTE}
	if !forceDispositionFallback {
		err := windows.NtSetInformationFile(h, &iosb, (*byte)(unsafe.Pointer(&ex)), uint32(unsafe.Sizeof(ex)),
			windows.FileDispositionInformationEx)
		if err == nil {
			return nil
		}
		var st windows.NTStatus
		if !errors.As(err, &st) || (st != windows.STATUS_INVALID_INFO_CLASS && st != windows.STATUS_INVALID_PARAMETER &&
			st != windows.STATUS_NOT_SUPPORTED) {
			return ntErrno(err)
		}
	}

	cleared := false
	if attrs&windows.FILE_ATTRIBUTE_READONLY != 0 && attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		if err := setAttrs(h, attrs&^windows.FILE_ATTRIBUTE_READONLY); err != nil {
			return err
		}
		cleared = true
	}
	d := fileDispositionInfo{DeleteFile: 1}
	err := windows.SetFileInformationByHandle(h, windows.FileDispositionInfo, (*byte)(unsafe.Pointer(&d)), uint32(unsafe.Sizeof(d)))
	if err != nil && cleared {
		_ = setAttrs(h, attrs)
	}
	return err
}

// setAttrs sets the attributes of the object behind h (never a path).
func setAttrs(h windows.Handle, attrs uint32) error {
	attrs &^= windows.FILE_ATTRIBUTE_DIRECTORY | windows.FILE_ATTRIBUTE_REPARSE_POINT
	if attrs == 0 {
		attrs = windows.FILE_ATTRIBUTE_NORMAL
	}
	bi := fileBasicInfo{FileAttributes: attrs}
	return windows.SetFileInformationByHandle(h, windows.FileBasicInfo, (*byte)(unsafe.Pointer(&bi)), uint32(unsafe.Sizeof(bi)))
}

// fileIDBothDirInfo is FILE_ID_BOTH_DIR_INFO (the Go layout matches the C
// one: FileId at offset 96, FileName at 104).
type fileIDBothDirInfo struct {
	NextEntryOffset uint32
	FileIndex       uint32
	CreationTime    int64
	LastAccessTime  int64
	LastWriteTime   int64
	ChangeTime      int64
	EndOfFile       int64
	AllocationSize  int64
	FileAttributes  uint32
	FileNameLength  uint32 // bytes
	EaSize          uint32
	ShortNameLength int8
	ShortName       [12]uint16
	FileID          int64
	FileName        [1]uint16
}

// listHandle returns the names in the directory behind h, read through the
// handle (GetFileInformationByHandleEx), excluding "." and "..".
func listHandle(h windows.Handle) ([]string, error) {
	buf := make([]uint64, 8192) // 64 KiB, 8-byte aligned
	var names []string
	for {
		err := windows.GetFileInformationByHandleEx(h, windows.FileIdBothDirectoryInfo,
			(*byte)(unsafe.Pointer(&buf[0])), uint32(len(buf)*8))
		if err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				return names, nil
			}
			return nil, err
		}
		base := unsafe.Pointer(&buf[0])
		for off := uintptr(0); ; {
			e := (*fileIDBothDirInfo)(unsafe.Add(base, off))
			n := unsafe.Slice(&e.FileName[0], e.FileNameLength/2)
			if name := windows.UTF16ToString(n); name != "." && name != ".." {
				names = append(names, name)
			}
			if e.NextEntryOffset == 0 {
				break
			}
			off += uintptr(e.NextEntryOffset)
		}
	}
}
