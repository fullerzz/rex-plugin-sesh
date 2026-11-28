//go:build windows

package config

import "strings"

// shellQuote quotes s for cmd.exe and the program it starts. A Windows path
// can't contain a double quote, so double quotes keep spaces and & | < > ^
// literal. Trailing backslashes are doubled, so a path such as C:\ doesn't
// escape the closing quote.
func shellQuote(s string) string {
	trimmed := strings.TrimRight(s, `\`)
	return `"` + s + strings.Repeat(`\`, len(s)-len(trimmed)) + `"`
}
