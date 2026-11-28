//go:build !windows

package config

import "strings"

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool {
		return !isShellSafeRune(r)
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func isShellSafeRune(r rune) bool {
	return r >= 'A' && r <= 'Z' ||
		r >= 'a' && r <= 'z' ||
		r >= '0' && r <= '9' ||
		strings.ContainsRune("/_:.,-+", r)
}
