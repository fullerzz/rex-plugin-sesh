package sources

import (
	"bufio"
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fullerzz/rex-plugin-sesh/internal/model"
)

type Zoxide struct {
	Runner func(context.Context) (string, error)
}

func (Zoxide) Name() string { return "zoxide" }
func (z Zoxide) List(ctx context.Context) (model.Sessions, error) {
	out := model.NewSessions()
	var raw string
	var err error
	if z.Runner != nil {
		raw, err = z.Runner(ctx)
	} else {
		b, e := exec.CommandContext(ctx, "zoxide", "query", "-l", "-s").Output()
		raw = string(b)
		err = e
	}
	if err != nil {
		return out, nil
	}
	var dirs []model.Session
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		if s, ok := ParseZoxideLine(scanner.Text()); ok {
			dirs = append(dirs, s)
		}
	}
	paths := make([]string, len(dirs))
	for i, d := range dirs {
		paths[i] = d.Path
	}
	for i, name := range uniqueNames(paths) {
		dirs[i].Name = name
		out.Add(dirs[i])
	}
	return out, nil
}

// uniqueNames names each directory after its last path element. Directories
// that would share a name get parent elements added until each name is
// unique, so /a/docs and /b/docs become a/docs and b/docs.
func uniqueNames(paths []string) []string {
	depth := make([]int, len(paths))
	for i := range depth {
		depth[i] = 1
	}
	names := make([]string, len(paths))
	for {
		count := make(map[string]int, len(paths))
		for i, p := range paths {
			names[i] = lastElements(p, depth[i])
			count[names[i]]++
		}
		grew := false
		for i, name := range names {
			if count[name] > 1 && depth[i] < len(pathElements(paths[i])) {
				depth[i]++
				grew = true
			}
		}
		if !grew {
			return names
		}
	}
}

func pathElements(path string) []string {
	return strings.FieldsFunc(filepath.Clean(path), func(r rune) bool { return r == filepath.Separator })
}

// lastElements joins the last n elements of path with "/". With n of 1 it is
// filepath.Base, which also names the root.
func lastElements(path string, n int) string {
	elems := pathElements(path)
	if n <= 1 || len(elems) < 2 {
		return filepath.Base(path)
	}
	n = min(n, len(elems))
	return strings.Join(elems[len(elems)-n:], "/")
}
func ParseZoxideLine(line string) (model.Session, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return model.Session{}, false
	}
	fields := strings.Fields(line)
	score := 0.0
	path := line
	if len(fields) >= 2 {
		if f, err := strconv.ParseFloat(fields[0], 64); err == nil {
			score = f
			path = strings.Join(fields[1:], " ")
		}
	}
	return model.Session{Source: "zoxide", Name: filepath.Base(path), Path: path, Score: score}, true
}
func AddPath(ctx context.Context, path string) {
	//nolint:gosec // zoxide is a fixed local integration; path is a single argv value.
	_ = exec.CommandContext(ctx, "zoxide", "add", path).Run()
}
