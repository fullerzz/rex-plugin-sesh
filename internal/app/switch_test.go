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

func TestSwitchRefusesRowsItCannotOpen(t *testing.T) {
	err := (&App{}).switchTo(t.Context(), switchTarget{}, model.Session{Source: "config", Name: "web"})
	require.EqualError(t, err, `"web" is not a Rex session or a directory on this machine`)
	err = (&App{}).switchTo(t.Context(), switchTarget{}, model.Session{Source: "config", Name: "web", Path: "/src/web", Machine: "devbox"})
	require.EqualError(t, err, `"[devbox] web" is not a Rex session or a directory on this machine`)
	err = (&App{}).switchTo(t.Context(), switchTarget{}, model.Session{Source: "rex", Name: "web"})
	require.EqualError(t, err, `"web" is not an open Rex session`)
}

// fakeRexNew is fakeRexWithApp that also answers rex new.
const fakeRexNew = `printf '%s\n' "$*" >> "$FAKE_REX_LOG"
case "$*" in
  *"client ls"*) printf '%s' '{"clients":[{"client_id":"client:app","info":{"kind":"app"}}]}' ;;
  *" new "*) printf '%s' '{"session_id":"session:new"}' ;;
  *"session.select"*) if [ -n "$FAKE_REX_REFUSE" ]; then printf 'Error: %s\n' "$FAKE_REX_REFUSE" >&2; exit 1; fi ;;
  *) printf '%s' '{"sessions":{}}' ;;
esac`

// useFakeZoxide puts a zoxide on PATH that logs its arguments to the returned
// file.
func useFakeZoxide(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "zoxide.log")
	//nolint:gosec // the fake zoxide must be executable.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zoxide"), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> '"+log+"'\n"), 0o700))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func callsMatching(t *testing.T, log, substr string) []string {
	t.Helper()
	//nolint:gosec // log is in this test's temporary directory.
	data, err := os.ReadFile(log)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	require.NoError(t, err)
	var calls []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.Contains(line, substr) {
			calls = append(calls, line)
		}
	}
	return calls
}

// Choosing a directory creates a session there, named after the row, and
// switches to it. A retry after a refusal selects that session again instead
// of creating a second one, and zoxide learns the directory only on success.
func TestSwitchOpensADirectoryOnce(t *testing.T) {
	log := filepath.Join(t.TempDir(), "rex.log")
	t.Setenv("FAKE_REX_LOG", log)
	useFakeRex(t, fakeRexNew)
	zoxideLog := useFakeZoxide(t)
	dir := t.TempDir()
	row := model.Session{Source: "zoxide", Name: "my api", Path: dir}
	backend := &pickerBackend{app: &App{}, ctx: t.Context(), cfg: config.Default()}

	t.Setenv("FAKE_REX_REFUSE", "Remote Control is off")
	require.EqualError(t, backend.Switch(t.Context(), row), "rex session.select: Remote Control is off")
	assert.Empty(t, callsMatching(t, zoxideLog, "add"))
	t.Setenv("FAKE_REX_REFUSE", "")
	require.NoError(t, backend.Switch(t.Context(), row))

	assert.Equal(t, []string{"--autostart=false new --cwd " + dir + " --json my api"}, callsMatching(t, log, " new "))
	assert.Equal(t, []string{
		"--autostart=false -C client:app do session.select session_id=session:new",
		"--autostart=false -C client:app do session.select session_id=session:new",
	}, selectCalls(t, log))
	assert.Equal(t, []string{"add " + dir}, callsMatching(t, zoxideLog, "add"))
}

// Without an app to switch, nothing is created.
func TestSwitchCreatesNothingWithoutAnApp(t *testing.T) {
	log := filepath.Join(t.TempDir(), "rex.log")
	t.Setenv("FAKE_REX_LOG", log)
	useFakeRex(t, `printf '%s\n' "$*" >> "$FAKE_REX_LOG"; printf '%s' '{"clients":[]}'`)
	err := (&App{}).switchTo(t.Context(), switchTarget{}, model.Session{Source: "zoxide", Name: "api", Path: t.TempDir()})
	require.EqualError(t, err, "no Rex app is connected to switch")
	assert.Empty(t, callsMatching(t, log, " new "))
}

func TestSwitchReportsAMissingDirectory(t *testing.T) {
	log := filepath.Join(t.TempDir(), "rex.log")
	t.Setenv("FAKE_REX_LOG", log)
	useFakeRex(t, fakeRexNew)
	gone := filepath.Join(t.TempDir(), "gone")
	err := (&App{}).switchTo(t.Context(), switchTarget{}, model.Session{Source: "zoxide", Name: "gone", Path: gone})
	require.EqualError(t, err, gone+" is not a directory on this machine")
	assert.Empty(t, callsMatching(t, log, " new "))
}
