//go:build windows

package cleaner

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

var (
	clsidFileOperation = ole.NewGUID("{3AD05575-8857-4850-9277-11B85BDB8E09}")
	iidIFileOperation  = ole.NewGUID("{947AAB5F-0A5C-4C13-B4D6-4BF7836FC9F8}")
	iidIShellItem      = ole.NewGUID("{43826D1E-E718-42EE-BC55-A1E261C37BFE}")

	procSHCreateItemFromParsingName = windows.NewLazySystemDLL("shell32.dll").NewProc("SHCreateItemFromParsingName")
)

// IFileOperation operation flags.
const (
	fofSilent           = 0x0004
	fofNoConfirmation   = 0x0010
	fofNoErrorUI        = 0x0400
	fofWantNukeWarning  = 0x4000 // warn (and let the user abort) if an item cannot be recycled
	fofxRecycleOnDelete = 0x00080000

	recycleFlags = fofxRecycleOnDelete | fofNoConfirmation | fofSilent | fofNoErrorUI | fofWantNukeWarning
)

// HRESULTs of a user-cancelled operation.
const (
	hrErrorCancelled          = 0x800704C7 // HRESULT_FROM_WIN32(ERROR_CANCELLED)
	hrCopyEngineUserCancelled = 0x80270000 // COPYENGINE_E_USER_CANCELLED
)

// Vtable slots (IUnknown: 0 QueryInterface, 1 AddRef, 2 Release).
const (
	vtRelease                 = 2
	vtSetOperationFlags       = 5
	vtDeleteItem              = 18
	vtPerformOperations       = 21
	vtGetAnyOperationsAborted = 22
)

// Recycle moves path to the recycle bin through IFileOperation, on a
// dedicated OS thread with its own single-threaded COM apartment. The shell
// does not accept \\?\ paths, so paths longer than MAX_PATH fail with
// ErrPathTooLong. An aborted operation yields ErrRecycleAborted; an item
// still present after a reported success is an error.
func (osDeleter) Recycle(path string) error {
	abs, err := filepath.Abs(stripExtended(path))
	if err != nil {
		return &fs.PathError{Op: "recycle", Path: path, Err: err}
	}
	u16, err := windows.UTF16FromString(abs)
	if err != nil {
		return &fs.PathError{Op: "recycle", Path: path, Err: err}
	}
	if len(u16) > windows.MAX_PATH { // includes the terminating NUL
		return fmt.Errorf("%w: %s", ErrPathTooLong, abs)
	}

	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		done <- recycleSTA(&u16[0])
	}()
	if err := <-done; err != nil {
		return &fs.PathError{Op: "recycle", Path: abs, Err: err}
	}

	ext, err := windows.UTF16PtrFromString(extendedPath(abs))
	if err != nil {
		return err
	}
	if _, err := windows.GetFileAttributes(ext); err == nil {
		return fmt.Errorf("cleaner: %s still exists after recycling", abs)
	} else if !isNotExist(err) {
		return fmt.Errorf("cleaner: cannot verify that %s was recycled: %w", abs, err)
	}
	return nil
}

// recycleSTA runs on a locked OS thread.
func recycleSTA(path *uint16) error {
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		var oe *ole.OleError
		if !errors.As(err, &oe) || oe.Code() != 1 { // S_FALSE: already initialized, still balanced below
			return fmt.Errorf("CoInitializeEx: %w", err)
		}
	}
	defer ole.CoUninitialize()

	unk, err := ole.CreateInstance(clsidFileOperation, iidIFileOperation)
	if err != nil {
		return fmt.Errorf("CoCreateInstance(FileOperation): %w", err)
	}
	op := unsafe.Pointer(unk)
	defer vcall(op, vtRelease)

	var item unsafe.Pointer
	hr, _, _ := procSHCreateItemFromParsingName.Call(uintptr(unsafe.Pointer(path)), 0,
		uintptr(unsafe.Pointer(iidIShellItem)), uintptr(unsafe.Pointer(&item)))
	if err := hresult("SHCreateItemFromParsingName", hr); err != nil {
		return err
	}
	defer vcall(item, vtRelease)

	if err := hresult("SetOperationFlags", vcall(op, vtSetOperationFlags, recycleFlags)); err != nil {
		return err
	}
	if err := hresult("DeleteItem", vcall(op, vtDeleteItem, uintptr(item), 0)); err != nil {
		return err
	}
	phr := vcall(op, vtPerformOperations)
	var aborted int32
	if err := hresult("GetAnyOperationsAborted", vcall(op, vtGetAnyOperationsAborted, uintptr(unsafe.Pointer(&aborted)))); err == nil && aborted != 0 {
		return ErrRecycleAborted
	}
	if h := uint32(phr); h == hrErrorCancelled || h == hrCopyEngineUserCancelled {
		return ErrRecycleAborted
	}
	return hresult("PerformOperations", phr)
}

// vcall calls vtable slot idx of COM object obj with obj as first argument.
func vcall(obj unsafe.Pointer, idx int, args ...uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(vtbl, idx*int(unsafe.Sizeof(uintptr(0)))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(obj)}, args...)...)
	return r
}

// hresult turns a failed HRESULT into an error; Win32 not-found codes wrap
// fs.ErrNotExist.
func hresult(op string, hr uintptr) error {
	h := uint32(hr)
	if int32(h) >= 0 {
		return nil
	}
	if h&0xFFFF0000 == 0x80070000 {
		errno := windows.Errno(h & 0xFFFF)
		return fmt.Errorf("%s: %w", op, errno) // Errno matches fs.ErrNotExist for not-found codes
	}
	return fmt.Errorf("%s: HRESULT 0x%08X", op, h)
}
