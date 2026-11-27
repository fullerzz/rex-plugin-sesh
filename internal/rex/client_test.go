package rex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRunner struct {
	stdout, stderr string
	err            error

	bin   string
	args  []string
	stdin []byte
}

func (f *fakeRunner) Run(_ context.Context, bin string, args []string, stdin []byte) ([]byte, []byte, error) {
	f.bin, f.args, f.stdin = bin, args, stdin
	return []byte(f.stdout), []byte(f.stderr), f.err
}

func TestSessionsRunsEmbeddedScriptWithoutAutostart(t *testing.T) {
	runner := &fakeRunner{stdout: `{"sessions":[]}`}
	_, err := (&Client{Bin: "/opt/rex", Runner: runner}).Sessions(t.Context(), "block:7")
	require.NoError(t, err)
	assert.Equal(t, "/opt/rex", runner.bin)
	assert.Equal(t, []string{"--autostart=false", "do", "-", "--args", `{"block_id":"block:7"}`, "--json"}, runner.args)
	assert.Equal(t, sessionsScript, runner.stdin)
}

func TestSessionsDecodesListAndPaths(t *testing.T) {
	runner := &fakeRunner{stdout: `{"sessions":[
		{"session_id":"session:1","label":"api","windows":2,"block_id":"block:9","pwd":"file://mac.local/Users/me/src/my%20api","agent_status":"blocked"},
		{"session_id":"session:2","label":"scratch","windows":0,"current":true}
	]}`}
	got, err := (&Client{Bin: "rex", Runner: runner}).Sessions(t.Context(), "")
	require.NoError(t, err)
	assert.Equal(t, []Session{
		{ID: "session:1", Label: "api", Windows: 2, BlockID: "block:9", Path: "/Users/me/src/my api", AgentStatus: "blocked"},
		{ID: "session:2", Label: "scratch", Current: true},
	}, got)
}

// Lua cannot tell an empty array from an empty object, so rex prints {}.
func TestSessionsAcceptsEmptyTableAsNoSessions(t *testing.T) {
	for _, stdout := range []string{`{"sessions":{}}`, `{}`, `{"sessions":null}`} {
		got, err := (&Client{Bin: "rex", Runner: &fakeRunner{stdout: stdout}}).Sessions(t.Context(), "")
		require.NoError(t, err, stdout)
		assert.Empty(t, got, stdout)
	}
}

func TestSessionsReportsRexError(t *testing.T) {
	runner := &fakeRunner{
		stderr: "Error: no Rex server is running at /tmp/rex.sock, and autostart is disabled\n",
		err:    errors.New("exit status 4"),
	}
	_, err := (&Client{Bin: "rex", Runner: runner}).Sessions(t.Context(), "")
	require.EqualError(t, err, "rex do: no Rex server is running at /tmp/rex.sock, and autostart is disabled")
}

func TestSessionsReportsMalformedOutput(t *testing.T) {
	_, err := (&Client{Bin: "rex", Runner: &fakeRunner{stdout: "not json"}}).Sessions(t.Context(), "")
	require.ErrorContains(t, err, "decode rex sessions")
}

func TestPathFromFileURL(t *testing.T) {
	for raw, want := range map[string]string{
		"file://mac.local/Users/me/src": "/Users/me/src",
		"file:///tmp/a%20b":             "/tmp/a b",
		"":                              "",
		"/Users/me":                     "",
		"https://example.com/x":         "",
		"file://host":                   "",
	} {
		assert.Equal(t, want, PathFromFileURL(raw), raw)
	}
}

func TestBinPrefersOverrideThenServerBinary(t *testing.T) {
	t.Setenv("REX_SESH_REX_BIN", "/custom/rex")
	assert.Equal(t, "/custom/rex", Bin())

	dir := t.TempDir()
	t.Setenv("REX_SESH_REX_BIN", "")
	t.Setenv("REX_BIN_DIR", dir)
	assert.Equal(t, "rex", Bin(), "REX_BIN_DIR without rex in it")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rex"), nil, 0o600))
	assert.Equal(t, filepath.Join(dir, "rex"), Bin())

	t.Setenv("REX_BIN_DIR", "")
	assert.Equal(t, "rex", Bin())
}

func TestAppClientsKeepsOnlyApps(t *testing.T) {
	runner := &fakeRunner{stdout: `{"clients":[
		{"client_id":"client:app","info":{"kind":"app","name":"Rex","label":"Studio Mac"}},
		{"client_id":"client:cli","info":{"kind":"cli"}},
		{"client_id":"client:bare"}
	]}`}
	got, err := (&Client{Bin: "rex", Runner: runner}).AppClients(t.Context(), "session:1")
	require.NoError(t, err)
	assert.Equal(t, []AppClient{{ID: "client:app", Label: "Studio Mac"}}, got)
	assert.Equal(t, []string{"--autostart=false", "client", "ls", "--json", "-s", "session:1"}, runner.args)

	_, err = (&Client{Bin: "rex", Runner: runner}).AppClients(t.Context(), "")
	require.NoError(t, err)
	assert.Equal(t, []string{"--autostart=false", "client", "ls", "--json"}, runner.args)
}

func TestSelectSessionAsksTheApp(t *testing.T) {
	runner := &fakeRunner{}
	require.NoError(t, (&Client{Bin: "rex", Runner: runner}).SelectSession(t.Context(), "client:app", "session:2"))
	assert.Equal(t, []string{"--autostart=false", "-C", "client:app", "do", "session.select", "session_id=session:2"}, runner.args)
}

func TestSelectSessionReportsRefusal(t *testing.T) {
	runner := &fakeRunner{stderr: "Error: Remote Control is off\n", err: errors.New("exit status 1")}
	err := (&Client{Bin: "rex", Runner: runner}).SelectSession(t.Context(), "client:app", "session:2")
	require.EqualError(t, err, "rex session.select: Remote Control is off")
}

func TestNewSessionCreatesInTheDirectory(t *testing.T) {
	runner := &fakeRunner{stdout: `{"session_id":"session:9","initial_windows":[]}`}
	got, err := (&Client{Bin: "rex", Runner: runner}).NewSession(t.Context(), "api", "/src/my api")
	require.NoError(t, err)
	assert.Equal(t, "session:9", got)
	assert.Equal(t, []string{"--autostart=false", "new", "--cwd", "/src/my api", "--json", "api"}, runner.args)
}

func TestNewSessionLeavesOutALabelThatReadsAsAFlag(t *testing.T) {
	runner := &fakeRunner{stdout: `{"session_id":"session:9"}`}
	_, err := (&Client{Bin: "rex", Runner: runner}).NewSession(t.Context(), "-rf", "/src/-rf")
	require.NoError(t, err)
	assert.Equal(t, []string{"--autostart=false", "new", "--cwd", "/src/-rf", "--json"}, runner.args)
}

func TestNewSessionRejectsAReplyWithoutAnID(t *testing.T) {
	_, err := (&Client{Bin: "rex", Runner: &fakeRunner{stdout: `{"sessions":{}}`}}).NewSession(t.Context(), "api", "/src")
	require.EqualError(t, err, "rex new: no session ID in the reply")
	runner := &fakeRunner{stderr: "Error: no such directory\n", err: errors.New("exit status 1")}
	_, err = (&Client{Bin: "rex", Runner: runner}).NewSession(t.Context(), "api", "/gone")
	require.EqualError(t, err, "rex new: no such directory")
}

func TestScreenKeepsTheVisibleLines(t *testing.T) {
	runner := &fakeRunner{stdout: `{"rows":2,"content":"old\r\n\u001b[32mtwo\u001b[0m\r\nthree\r\n\u001b[0m   \r\n\r\n"}`}
	got, err := (&Client{Bin: "rex", Runner: runner}).Screen(t.Context(), "session:1", "block:9")
	require.NoError(t, err)
	assert.Equal(t, "\x1b[32mtwo\x1b[0m\nthree\n", got)
	assert.Equal(t, []string{"--autostart=false", "do", "-", "--args", `{"block_id":"block:9","session_id":"session:1"}`, "--json"}, runner.args)
	assert.Equal(t, screenScript, runner.stdin)
}

func TestVisibleLinesHandlesShortAndEmptyScreens(t *testing.T) {
	assert.Equal(t, "a\nb\n", visibleLines("a\nb\n\n", 10))
	assert.Empty(t, visibleLines("\n  \n", 5))
	assert.Equal(t, "a\nb\n", visibleLines("a\nb", 0), "rows < 1 keeps every line")
}

func TestScreenReportsRexError(t *testing.T) {
	runner := &fakeRunner{stderr: "Error: block not found\n", err: errors.New("exit status 1")}
	_, err := (&Client{Bin: "rex", Runner: runner}).Screen(t.Context(), "session:1", "block:9")
	require.EqualError(t, err, "rex capture: block not found")
}

func TestHostClientPassesTheServer(t *testing.T) {
	runner := &fakeRunner{stdout: `{"sessions":[]}`}
	_, err := (&Client{Bin: "rex", Runner: runner, Server: "devbox"}).Sessions(t.Context(), "")
	require.NoError(t, err)
	assert.Equal(t, []string{"--autostart=false", "-S", "devbox", "do", "-", "--args", `{"block_id":""}`, "--json"}, runner.args)
}

func TestHostsListsLabelsInOrder(t *testing.T) {
	runner := &fakeRunner{stdout: `{"home":{"this_machine":true},"hosts":[
		{"host":{"label":"devbox","endpoints":["https://devbox.example"]},"from":"home"},
		{"host":{"label":""}},
		{"host":{"label":"lab"}}
	]}`}
	got, err := (&Client{Bin: "rex", Runner: runner}).Hosts(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{"devbox", "lab"}, got)
	assert.Equal(t, []string{"--autostart=false", "hosts", "--json"}, runner.args)

	got, err = (&Client{Bin: "rex", Runner: &fakeRunner{stdout: `{"sessions":{}}`}}).Hosts(t.Context())
	require.NoError(t, err)
	assert.Empty(t, got, "a reply without hosts lists none")
}
