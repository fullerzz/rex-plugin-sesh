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
	_, err := (&Client{Bin: "/opt/rex", Runner: runner}).Sessions(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "/opt/rex", runner.bin)
	assert.Equal(t, []string{"--autostart=false", "do", "-", "--json"}, runner.args)
	assert.Equal(t, sessionsScript, runner.stdin)
}

func TestSessionsDecodesListAndPaths(t *testing.T) {
	runner := &fakeRunner{stdout: `{"sessions":[
		{"session_id":"session:1","label":"api","windows":2,"focused_block_id":"block:9","pwd":"file://mac.local/Users/me/src/my%20api"},
		{"session_id":"session:2","label":"scratch","windows":0}
	]}`}
	got, err := (&Client{Bin: "rex", Runner: runner}).Sessions(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []Session{
		{ID: "session:1", Label: "api", Windows: 2, FocusedBlockID: "block:9", Path: "/Users/me/src/my api"},
		{ID: "session:2", Label: "scratch"},
	}, got)
}

// Lua cannot tell an empty array from an empty object, so rex prints {}.
func TestSessionsAcceptsEmptyTableAsNoSessions(t *testing.T) {
	for _, stdout := range []string{`{"sessions":{}}`, `{}`, `{"sessions":null}`} {
		got, err := (&Client{Bin: "rex", Runner: &fakeRunner{stdout: stdout}}).Sessions(t.Context())
		require.NoError(t, err, stdout)
		assert.Empty(t, got, stdout)
	}
}

func TestSessionsReportsRexError(t *testing.T) {
	runner := &fakeRunner{
		stderr: "Error: no Rex server is running at /tmp/rex.sock, and autostart is disabled\n",
		err:    errors.New("exit status 4"),
	}
	_, err := (&Client{Bin: "rex", Runner: runner}).Sessions(t.Context())
	require.EqualError(t, err, "rex do - --json: no Rex server is running at /tmp/rex.sock, and autostart is disabled")
}

func TestSessionsReportsMalformedOutput(t *testing.T) {
	_, err := (&Client{Bin: "rex", Runner: &fakeRunner{stdout: "not json"}}).Sessions(t.Context())
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
