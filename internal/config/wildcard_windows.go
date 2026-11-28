//go:build windows

package config

import "strings"

// PreviewPathEnv holds the selected path for a Windows preview command.
const PreviewPathEnv = "REX_SESH_PREVIEW_PATH"

// cmd.exe expands %NAME% even inside double quotes, but it expands the
// command line only once, so a path passed in PreviewPathEnv stays literal.
// A Windows path can't contain a double quote, so the quotes keep spaces and
// & | < > ^ literal. Trailing backslashes are doubled, so a path such as C:\
// doesn't escape the closing quote for the program cmd.exe starts.
func previewCommand(command, path string) (string, []string) {
	trailing := len(path) - len(strings.TrimRight(path, `\`))
	value := path + strings.Repeat(`\`, trailing)
	return strings.ReplaceAll(command, "{}", `"%`+PreviewPathEnv+`%"`), []string{PreviewPathEnv + "=" + value}
}
