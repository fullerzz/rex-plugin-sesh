package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeApps struct {
	all, attached []string
	err           error
	calls         []string
}

func (f *fakeApps) AppClients(_ context.Context, sessionID string) ([]string, error) {
	f.calls = append(f.calls, sessionID)
	if sessionID != "" {
		return f.attached, f.err
	}
	return f.all, f.err
}

func TestResolveClientPrefersFlagThenEnv(t *testing.T) {
	apps := &fakeApps{all: []string{"client:a", "client:b"}}
	t.Setenv("REX_CLIENT", "client:env")
	got, err := resolveClient(t.Context(), apps, switchTarget{client: "client:flag"})
	require.NoError(t, err)
	assert.Equal(t, "client:flag", got)
	got, err = resolveClient(t.Context(), apps, switchTarget{})
	require.NoError(t, err)
	assert.Equal(t, "client:env", got)
	assert.Empty(t, apps.calls, "a named client needs no lookup")
}

func TestResolveClientUsesTheOnlyApp(t *testing.T) {
	got, err := resolveClient(t.Context(), &fakeApps{all: []string{"client:a"}}, switchTarget{})
	require.NoError(t, err)
	assert.Equal(t, "client:a", got)
}

func TestResolveClientNarrowsToTheCurrentSession(t *testing.T) {
	apps := &fakeApps{all: []string{"client:a", "client:b"}, attached: []string{"client:b"}}
	got, err := resolveClient(t.Context(), apps, switchTarget{currentSessionID: "session:1"})
	require.NoError(t, err)
	assert.Equal(t, "client:b", got)
	assert.Equal(t, []string{"", "session:1"}, apps.calls)
}

func TestResolveClientReportsNoneOrSeveral(t *testing.T) {
	_, err := resolveClient(t.Context(), &fakeApps{}, switchTarget{})
	require.EqualError(t, err, "no Rex app is connected to switch")
	_, err = resolveClient(t.Context(), &fakeApps{all: []string{"client:a", "client:b"}, attached: []string{"client:a", "client:b"}}, switchTarget{currentSessionID: "session:1"})
	require.EqualError(t, err, "2 Rex apps are connected; choose one with --client")
	_, err = resolveClient(t.Context(), &fakeApps{err: errors.New("no server")}, switchTarget{})
	require.EqualError(t, err, "no server")
}

// fakeRexWithApp answers client ls with one app and logs every call.
const fakeRexWithApp = `printf '%s\n' "$*" >> "$FAKE_REX_LOG"
case "$*" in
  *"client ls"*) printf '%s' '{"clients":[{"client_id":"client:app","info":{"kind":"app"}},{"client_id":"client:cli","info":{"kind":"cli"}}]}' ;;
  *"session.select"*) if [ -n "$FAKE_REX_REFUSE" ]; then printf 'Error: %s\n' "$FAKE_REX_REFUSE" >&2; exit 1; fi ;;
  *) printf '%s' '{"sessions":{}}' ;;
esac`

func selectCalls(t *testing.T, log string) []string {
	t.Helper()
	//nolint:gosec // log is in this test's temporary directory.
	data, err := os.ReadFile(log)
	require.NoError(t, err)
	var calls []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.Contains(line, "session.select") {
			calls = append(calls, line)
		}
	}
	return calls
}

func TestPickerBackendSwitchSelectsOnce(t *testing.T) {
	log := filepath.Join(t.TempDir(), "rex.log")
	t.Setenv("FAKE_REX_LOG", log)
	useFakeRex(t, fakeRexWithApp)
	backend := &pickerBackend{app: &App{}, ctx: t.Context(), cfg: config.Default()}
	require.NoError(t, backend.Switch(t.Context(), model.Session{Source: "rex", Name: "web", SessionID: "session:2"}))
	assert.Equal(t, []string{"--autostart=false -C client:app do session.select session_id=session:2"}, selectCalls(t, log))
}

func TestPickerBackendSwitchReportsRefusal(t *testing.T) {
	t.Setenv("FAKE_REX_LOG", filepath.Join(t.TempDir(), "rex.log"))
	t.Setenv("FAKE_REX_REFUSE", "Remote Control is off")
	useFakeRex(t, fakeRexWithApp)
	backend := &pickerBackend{app: &App{}, ctx: t.Context(), cfg: config.Default(), target: switchTarget{client: "client:named"}}
	err := backend.Switch(t.Context(), model.Session{Source: "rex", Name: "web", SessionID: "session:2"})
	require.EqualError(t, err, "rex session.select: Remote Control is off")
	assert.Equal(t, []string{"--autostart=false -C client:named do session.select session_id=session:2"}, selectCalls(t, os.Getenv("FAKE_REX_LOG")))
}

func TestSwitchRefusesNonRexRows(t *testing.T) {
	err := (&App{}).switchTo(t.Context(), switchTarget{}, model.Session{Source: "config", Name: "web"})
	require.EqualError(t, err, `"web" is not an open Rex session`)
}
