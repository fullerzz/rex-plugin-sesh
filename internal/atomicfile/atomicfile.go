// Package atomicfile replaces a file with a complete new copy.
package atomicfile

// Replace renames the complete file tmp over path, so a reader sees either
// the old or the new contents.
func Replace(tmp, path string) error { return replace(tmp, path) }
