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

// PreviewCommand returns command with each {} standing for path, and any
// environment entries its shell needs to read path: on Unix, path quoted for
// sh in place of {}; on Windows, a quoted reference to PreviewPathEnv, so
// cmd.exe can't expand %NAME% text in the path itself.
func PreviewCommand(command, path string) (string, []string) {
	return previewCommand(command, path)
}
