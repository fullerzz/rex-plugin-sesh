package config

import (
	"path/filepath"
	"strings"
)

func MatchWildcard(pattern, path, home string) bool {
	p := filepath.Clean(ExpandHome(pattern, home))
	target := filepath.Clean(ExpandHome(path, home))
	if strings.HasSuffix(p, string(filepath.Separator)+"**") {
		prefix := strings.TrimSuffix(p, string(filepath.Separator)+"**")
		return target == prefix || strings.HasPrefix(target, prefix+string(filepath.Separator))
	}
	ok, _ := filepath.Match(p, target)
	return ok
}

func FindWildcard(cfg Config, path, home string) (WildcardConfig, bool) {
	for _, w := range cfg.WildcardConfigs {
		if MatchWildcard(w.Pattern, path, home) {
			return w, true
		}
	}
	return WildcardConfig{}, false
}

// SubstitutePath replaces each {} in command with path, quoted for the shell
// that runs preview commands: sh on Unix, cmd.exe on Windows.
func SubstitutePath(command, path string) string {
	return strings.ReplaceAll(command, "{}", shellQuote(path))
}
