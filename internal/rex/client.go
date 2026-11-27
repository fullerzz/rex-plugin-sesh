// Package rex talks to a Rex server through the rex command-line tool.
package rex

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed sessions.lua
var sessionsScript []byte

// requestTimeout bounds one rex invocation.
const requestTimeout = 5 * time.Second

// Session is one Rex session as the picker lists it.
type Session struct {
	ID    string
	Label string
	// Windows counts the session's tabs.
	Windows int
	// FocusedBlockID is the focused block of the active tab, or "".
	FocusedBlockID string
	// Path is the directory the focused terminal last reported, or "".
	Path string
}

// Runner runs a command with stdin and returns its output.
type Runner interface {
	Run(ctx context.Context, bin string, args []string, stdin []byte) (stdout, stderr []byte, err error)
}

// ExecRunner runs commands as child processes.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, bin string, args []string, stdin []byte) ([]byte, []byte, error) {
	//nolint:gosec // bin is the rex CLI, chosen by Bin below; args are fixed.
	c := exec.CommandContext(ctx, bin, args...)
	c.WaitDelay = 100 * time.Millisecond
	c.Stdin = bytes.NewReader(stdin)
	var out, errb bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errb
	err := c.Run()
	return out.Bytes(), errb.Bytes(), err
}

// Client calls the rex CLI. Every call passes --autostart=false, so a missing
// server is an error instead of a new server.
type Client struct {
	Bin    string
	Runner Runner
}

// NewClient finds rex the way Rex terminals do: REX_SESH_REX_BIN, then the
// server's own binary in REX_BIN_DIR, then rex on PATH.
func NewClient() *Client {
	return &Client{Bin: Bin(), Runner: ExecRunner{}}
}

// Bin returns the rex binary NewClient uses.
func Bin() string {
	if bin := os.Getenv("REX_SESH_REX_BIN"); bin != "" {
		return bin
	}
	if dir := os.Getenv("REX_BIN_DIR"); dir != "" {
		bin := filepath.Join(dir, "rex")
		//nolint:gosec // REX_BIN_DIR is set by the Rex server that runs this terminal.
		if st, err := os.Stat(bin); err == nil && !st.IsDir() {
			return bin
		}
	}
	return "rex"
}

// Sessions lists every session on the server in rex ls order.
func (c *Client) Sessions(ctx context.Context) ([]Session, error) {
	out, err := c.run(ctx, sessionsScript, "do", "-", "--json")
	if err != nil {
		return nil, err
	}
	var result struct {
		// An empty Lua table encodes as {}, so an empty list may be an object.
		Sessions json.RawMessage `json:"sessions"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("decode rex sessions: %w", err)
	}
	var raw []struct {
		ID             string `json:"session_id"`
		Label          string `json:"label"`
		Windows        int    `json:"windows"`
		FocusedBlockID string `json:"focused_block_id"`
		PWD            string `json:"pwd"`
	}
	if trimmed := bytes.TrimSpace(result.Sessions); len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("{}")) && !bytes.Equal(trimmed, []byte("null")) {
		if err := json.Unmarshal(trimmed, &raw); err != nil {
			return nil, fmt.Errorf("decode rex sessions: %w", err)
		}
	}
	sessions := make([]Session, 0, len(raw))
	for _, r := range raw {
		sessions = append(sessions, Session{
			ID:             r.ID,
			Label:          r.Label,
			Windows:        r.Windows,
			FocusedBlockID: r.FocusedBlockID,
			Path:           PathFromFileURL(r.PWD),
		})
	}
	return sessions, nil
}

func (c *Client) run(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	if c == nil || c.Runner == nil {
		return nil, errors.New("rex client is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	argv := append([]string{"--autostart=false"}, args...)
	out, errb, err := c.Runner.Run(ctx, c.Bin, argv, stdin)
	if err != nil {
		if msg := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(errb)), "Error:")); msg != "" {
			return nil, fmt.Errorf("rex %s: %s", strings.Join(args, " "), msg)
		}
		return nil, fmt.Errorf("rex %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// PathFromFileURL returns the local path in a file:// URL that a terminal
// reported, such as file://host/Users/me/src. It returns "" for anything else.
func PathFromFileURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || !strings.HasPrefix(u.Path, "/") {
		return ""
	}
	return u.Path
}
