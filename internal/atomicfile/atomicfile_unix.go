//go:build !windows

package atomicfile

import "os"

func replace(tmp, path string, check func() error) error {
	if check != nil {
		if err := check(); err != nil {
			return err
		}
	}
	return os.Rename(tmp, path)
}
