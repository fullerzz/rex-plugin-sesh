//go:build windows

package atomicfile

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// Go opens files without FILE_SHARE_DELETE, so renaming over a file fails
// while another process, such as a second picker, is reading it. Reads are
// brief, so retry for a while before giving up.
const retryFor = 2 * time.Second

func replace(tmp, path string, check func() error) error {
	deadline := time.Now().Add(retryFor)
	for {
		if check != nil {
			if err := check(); err != nil {
				return err
			}
		}
		err := os.Rename(tmp, path)
		if err == nil || !busy(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func busy(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
