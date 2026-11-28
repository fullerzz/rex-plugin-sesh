package preview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/fullerzz/rex-plugin-sesh/internal/rex"
)

// ErrPaneUnavailable reports a session with no Rex terminal to show.
var ErrPaneUnavailable = errors.New("this session has no terminal to preview")

// screenReader reads a Rex block's visible screen; *rex.Client implements it.
type screenReader interface {
	Screen(ctx context.Context, sessionID, blockID string) (string, error)
}

// newScreenReader reads screens on the server of host, or on this machine's
// for host "". Tests replace it.
var newScreenReader = func(host string) screenReader {
	if host != "" {
		return rex.NewHostClient(host)
	}
	return rex.NewClient()
}

// RenderPane returns the visible screen of the session's preview block.
func RenderPane(ctx context.Context, s model.Session) (string, error) {
	if s.Source != "rex" || s.SessionID == "" || s.BlockID == "" {
		return "", ErrPaneUnavailable
	}
	return newScreenReader(s.Machine).Screen(ctx, s.SessionID, s.BlockID)
}

// Render previews s. A remote session's preview runs nothing and shows its
// metadata.
func Render(ctx context.Context, s model.Session, fallbackCommand string) (string, error) {
	if s.Machine != "" {
		return remoteSummary(s), nil
	}
	if s.Path == "" {
		if s.SessionID != "" {
			return fmt.Sprintf("session: %s\nid: %s\npath: %s\n", s.Name, s.SessionID, s.Path), nil
		}
		return "No item path available\n", nil
	}

	cmd := s.PreviewCommand
	if cmd == "" {
		cmd = fallbackCommand
	}
	if cmd != "" {
		command, env := config.PreviewCommand(cmd, s.Path)
		return runShell(ctx, command, env)
	}
	if s.SessionID != "" {
		return fmt.Sprintf("session: %s\nid: %s\npath: %s\n", s.Name, s.SessionID, s.Path), nil
	}
	return directoryFallback(s.Path)
}

func remoteSummary(s model.Session) string {
	if s.Source == "rex" {
		return fmt.Sprintf("session: %s\nhost: %s\nid: %s\npath: %s\n", s.Name, s.Machine, s.SessionID, s.Path)
	}
	return fmt.Sprintf("workspace: %s\nmachine: %s\nid: %s\npath: %s\n", s.Name, s.Machine, s.SessionID, s.Path)
}

// runShell runs a user's preview command through this platform's shell, with
// env added to its environment, and returns its output. It ends every process
// the command started, both when the command finishes and when ctx ends or
// the 5 second limit passes.
func runShell(ctx context.Context, command string, env []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var out, errb bytes.Buffer
	if err := runProcessTree(ctx, command, env, &out, &errb); err != nil {
		return out.String() + errb.String(), err
	}
	return out.String(), nil
}

func directoryFallback(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("no path to preview")
	}
	ents, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(ents))
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}
	sort.Strings(lines)
	return filepath.Clean(path) + "\n" + strings.Join(lines, "\n") + "\n", nil
}
