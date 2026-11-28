// Package filelock takes exclusive advisory locks on lock files. Each open
// lock file is its own lock owner: a second Open of the same path, in this
// process or another, cannot lock it while the first holds it.
package filelock

import "errors"

// ErrWouldBlock reports that another open file holds the lock.
var ErrWouldBlock = errors.New("lock is held by another process")
