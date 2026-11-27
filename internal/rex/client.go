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

	"github.com/charmbracelet/x/ansi"
)

//go:embed sessions.lua
var sessionsScript []byte

//go:embed screen.lua
var screenScript []byte

// requestTimeout bounds one rex invocation.
const requestTimeout = 5 * time.Second

// actionTimeout bounds a client action. rex waits up to 10 seconds for the app
// to answer, so killing rex sooner could report a switch that still happens.
const actionTimeout = 15 * time.Second

// Session is one Rex session as the picker lists it.
type Session struct {
	ID    string
	Label string
	// Windows counts the session's tabs.
	Windows int
	// BlockID is the block to preview: the active tab's focused block, unless
	// that is the caller's floating picker, or "".
	BlockID string
	// AgentStatus is the most urgent state any program in the session
	// reports (blocked, working, done or idle), or "".
	AgentStatus string
	// Path is the directory a terminal in the active tab last reported, or "".
	Path string
	// Current marks the session that holds the block passed to Sessions.
	Current bool
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
	// Server is the host label each call passes as -S, or "" for this
	// machine's server.
	Server string
}

// NewClient finds rex the way Rex terminals do: REX_SESH_REX_BIN, then the
// server's own binary in REX_BIN_DIR, then rex on PATH.
func NewClient() *Client {
	return &Client{Bin: Bin(), Runner: ExecRunner{}}
}

// NewHostClient is NewClient for the server of the host labelled server, as
// "rex hosts" lists it.
func NewHostClient(server string) *Client {
	return &Client{Bin: Bin(), Runner: ExecRunner{}, Server: server}
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

// Sessions lists every session on the server in rex ls order. selfBlockID,
// when set, is the caller's own block: its session is marked Current.
func (c *Client) Sessions(ctx context.Context, selfBlockID string) ([]Session, error) {
	args, err := json.Marshal(map[string]string{"block_id": selfBlockID})
	if err != nil {
		return nil, err
	}
	out, err := c.run(ctx, requestTimeout, sessionsScript, "do", "do", "-", "--args", string(args), "--json")
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
		ID          string `json:"session_id"`
		Label       string `json:"label"`
		Windows     int    `json:"windows"`
		BlockID     string `json:"block_id"`
		PWD         string `json:"pwd"`
		Current     bool   `json:"current"`
		AgentStatus string `json:"agent_status"`
	}
	if trimmed := bytes.TrimSpace(result.Sessions); len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("{}")) && !bytes.Equal(trimmed, []byte("null")) {
		if err := json.Unmarshal(trimmed, &raw); err != nil {
			return nil, fmt.Errorf("decode rex sessions: %w", err)
		}
	}
	sessions := make([]Session, 0, len(raw))
	for _, r := range raw {
		sessions = append(sessions, Session{
			ID:          r.ID,
			Label:       r.Label,
			Windows:     r.Windows,
			BlockID:     r.BlockID,
			Path:        PathFromFileURL(r.PWD),
			Current:     r.Current,
			AgentStatus: r.AgentStatus,
		})
	}
	return sessions, nil
}

// Screen returns the visible screen of a block as VT text: the last lines that
// fit the terminal's rows, without trailing blank lines.
func (c *Client) Screen(ctx context.Context, sessionID, blockID string) (string, error) {
	args, err := json.Marshal(map[string]string{"session_id": sessionID, "block_id": blockID})
	if err != nil {
		return "", err
	}
	out, err := c.run(ctx, requestTimeout, screenScript, "capture", "do", "-", "--args", string(args), "--json")
	if err != nil {
		return "", err
	}
	var result struct {
		Rows    int    `json:"rows"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", fmt.Errorf("decode rex screen: %w", err)
	}
	return visibleLines(result.Content, result.Rows), nil
}

// visibleLines keeps the last rows lines of content and drops trailing lines
// that show nothing. rows < 1 keeps every line.
func visibleLines(content string, rows int) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(ansi.Strip(lines[len(lines)-1])) == "" {
		lines = lines[:len(lines)-1]
	}
	if rows > 0 && len(lines) > rows {
		lines = lines[len(lines)-rows:]
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// AppClient is a Rex app connected to a server.
type AppClient struct {
	ID string
	// Label names the device the app runs on. An app connected to several
	// servers has a different ID on each, and the same label.
	Label string
}

// Hosts lists the labels of the other Rex servers this machine connects to,
// in "rex hosts" order. The command ignores Server.
func (c *Client) Hosts(ctx context.Context) ([]string, error) {
	out, err := c.run(ctx, requestTimeout, nil, "hosts", "hosts", "--json")
	if err != nil {
		return nil, err
	}
	var result struct {
		Hosts []struct {
			Host struct {
				Label string `json:"label"`
			} `json:"host"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("decode rex hosts: %w", err)
	}
	var labels []string
	for _, h := range result.Hosts {
		if h.Host.Label != "" {
			labels = append(labels, h.Host.Label)
		}
	}
	return labels, nil
}

// AppClients lists the connected Rex apps, leaving out command-line and other
// clients. With sessionID set, it lists only apps attached to that session.
func (c *Client) AppClients(ctx context.Context, sessionID string) ([]AppClient, error) {
	args := []string{"client", "ls", "--json"}
	if sessionID != "" {
		args = append(args, "-s", sessionID)
	}
	out, err := c.run(ctx, requestTimeout, nil, "client ls", args...)
	if err != nil {
		return nil, err
	}
	var result struct {
		Clients []struct {
			ID   string `json:"client_id"`
			Info struct {
				Kind  string `json:"kind"`
				Label string `json:"label"`
			} `json:"info"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("decode rex clients: %w", err)
	}
	var apps []AppClient
	for _, client := range result.Clients {
		if client.Info.Kind == "app" {
			apps = append(apps, AppClient{ID: client.ID, Label: client.Info.Label})
		}
	}
	return apps, nil
}

// InspectApp asks Rex to resolve a selector with its own ambiguity rules.
func (c *Client) InspectApp(ctx context.Context, selector string) (AppClient, error) {
	out, err := c.run(ctx, requestTimeout, nil, "client inspect", "client", "inspect", "--json", "--", selector)
	if err != nil {
		return AppClient{}, err
	}
	var result struct {
		Client struct {
			ID   string `json:"client_id"`
			Info struct {
				Kind  string `json:"kind"`
				Label string `json:"label"`
			} `json:"info"`
		} `json:"client"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return AppClient{}, fmt.Errorf("decode rex client: %w", err)
	}
	if result.Client.ID == "" || result.Client.Info.Kind != "app" {
		return AppClient{}, fmt.Errorf("no Rex app matches %q", selector)
	}
	return AppClient{ID: result.Client.ID, Label: result.Client.Info.Label}, nil
}

// NewSession creates a session whose first terminal starts in cwd, and returns
// its ID. Rex allows several sessions with one label. A label that starts with
// "-" would read as a flag, so Rex names that session itself.
func (c *Client) NewSession(ctx context.Context, label, cwd string) (string, error) {
	args := []string{"new", "--cwd", cwd, "--json"}
	if label != "" && !strings.HasPrefix(label, "-") {
		args = append(args, label)
	}
	out, err := c.run(ctx, requestTimeout, nil, "new", args...)
	if err != nil {
		return "", err
	}
	var result struct {
		ID string `json:"session_id"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", fmt.Errorf("decode rex new: %w", err)
	}
	if result.ID == "" {
		return "", errors.New("rex new: no session ID in the reply")
	}
	return result.ID, nil
}

// SelectSession asks the Rex app clientID to show sessionID. The app accepts
// it only while Remote Control is on in its settings.
func (c *Client) SelectSession(ctx context.Context, clientID, sessionID string) error {
	_, err := c.run(ctx, actionTimeout, nil, "session.select", "-C", clientID, "do", "session.select", "session_id="+sessionID)
	return err
}

// run runs rex with args, naming the call label in its errors.
func (c *Client) run(ctx context.Context, timeout time.Duration, stdin []byte, label string, args ...string) ([]byte, error) {
	if c == nil || c.Runner == nil {
		return nil, errors.New("rex client is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	argv := []string{"--autostart=false"}
	if c.Server != "" {
		argv = append(argv, "-S", c.Server)
	}
	argv = append(argv, args...)
	out, errb, err := c.Runner.Run(ctx, c.Bin, argv, stdin)
	if err != nil {
		if msg := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(errb)), "Error:")); msg != "" {
			return nil, fmt.Errorf("rex %s: %s", label, msg)
		}
		return nil, fmt.Errorf("rex %s: %w", label, err)
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
