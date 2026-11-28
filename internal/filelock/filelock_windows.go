//go:build windows

package filelock

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// Open opens or creates the lock file at path.
func Open(path string) (*os.File, error) {
	//nolint:gosec // callers pass lock paths inside their own state or config directory.
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
}

// OpenNoFollow is Open, but fails if path is a symbolic link, junction or
// other reparse point. It opens the path itself, not its target, and checks
// the open handle, so the path can't be swapped between check and open.
func OpenNoFollow(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	h, err := windows.CreateFile(name,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		_ = windows.CloseHandle(h)
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	const notFile = windows.FILE_ATTRIBUTE_REPARSE_POINT | windows.FILE_ATTRIBUTE_DIRECTORY
	if info.FileAttributes&notFile != 0 {
		_ = windows.CloseHandle(h)
		return nil, &os.PathError{Op: "open", Path: path, Err: fmt.Errorf("not a regular file (attributes %#x)", info.FileAttributes)}
	}
	return os.NewFile(uintptr(h), path), nil
}

// TryLock takes the lock without waiting. It returns an error matching
// ErrWouldBlock when another open file holds it.
func TryLock(f *os.File) error {
	var ol windows.Overlapped
	err := windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return errors.Join(ErrWouldBlock, err)
	}
	return err
}

// Unlock releases a lock taken with TryLock.
func Unlock(f *os.File) error {
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
