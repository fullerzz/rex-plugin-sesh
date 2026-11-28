// Package atomicfile replaces a file with a complete new copy.
package atomicfile

// Replace renames the complete file tmp over path, so a reader sees either
// the old or the new contents. When check is set, it runs before each
// attempt, and its error stops the replace: a retry can come after another
// process changed path.
func Replace(tmp, path string, check func() error) error { return replace(tmp, path, check) }
