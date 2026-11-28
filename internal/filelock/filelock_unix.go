//go:build unix

package filelock

import (
	"errors"
	"os"
	"syscall"
)

// Open opens or creates the lock file at path.
func Open(path string) (*os.File, error) {
	//nolint:gosec // callers pass lock paths inside their own state or config directory.
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
}

// OpenNoFollow is Open, but fails if path is a symbolic link.
func OpenNoFollow(path string) (*os.File, error) {
	//nolint:gosec // callers pass lock paths inside their own state or config directory.
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
}

// TryLock takes the lock without waiting. It returns an error matching
// ErrWouldBlock when another open file holds it.
func TryLock(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return errors.Join(ErrWouldBlock, err)
	}
	return err
}

// Unlock releases a lock taken with TryLock.
func Unlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
