//go:build !windows

package atomicfile

import "os"

func replace(tmp, path string) error { return os.Rename(tmp, path) }
