package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fullerzz/rex-plugin-sesh/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The config server and devbox have independent state and client IDs. Even
// their sessions can share an ID. The action reports devbox as the opener's
// host and executes last beside the config that registered the action.
func TestLastActionReturnsFromAnotherHostsSession(t *testing.T) {
	remoteState := useHistoryDir(t)
	recordSwitches(t, "id:client:devbox-app", [2]state.Visit{localVisit("unrelated"), localVisit("session:1")})
	before, err := os.ReadFile(state.HistoryPath(remoteState)) //nolint:gosec // test fixture
	require.NoError(t, err)
	useHistoryDir(t)
	log := filepath.Join(t.TempDir(), "rex.log")
	t.Setenv("FAKE_REX_LOG", log)
	useFakeRex(t, strings.NewReplacer(
		`"kind":"app"`, `"kind":"app","label":"Mac"`,
		`"session:9"`, `"session:1"`,
		`"label":"api"`, `"label":"api","current":true`,
	).Replace(fakeRexHosts))
	configureFakeSources(t, "")
	useFakeFZF(t, "[devbox] train")
	a := &App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	require.NoError(t, a.Run(t.Context(), []string{"picker", "--fzf", "--config", writeConfig(t, "version = 1\n"), "--opener", "client:app"}))

	require.NoError(t, a.Run(t.Context(), []string{"last", "--opener", "client:app", "--opener-session", "session:1", "--opener-host", "devbox"}))
	require.NoError(t, a.Run(t.Context(), []string{"last", "--opener", "client:app", "--opener-session", "session:1"}))
	assert.Equal(t, []string{
		"--autostart=false -S devbox -C client:devbox-app do session.select session_id=session:1",
		"--autostart=false -C client:app do session.select session_id=session:1",
		"--autostart=false -S devbox -C client:devbox-app do session.select session_id=session:1",
	}, selectCalls(t, log))
	after, err := os.ReadFile(state.HistoryPath(remoteState)) //nolint:gosec // test fixture
	require.NoError(t, err)
	assert.Equal(t, before, after)
}
