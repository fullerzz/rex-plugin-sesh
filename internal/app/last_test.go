package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	"github.com/fullerzz/rex-plugin-sesh/internal/rex"
	"github.com/fullerzz/rex-plugin-sesh/internal/settings"
	"github.com/fullerzz/rex-plugin-sesh/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLastTogglesBetweenPickerVisitsAndReloadsFooter(t *testing.T) {
	useHistoryDir(t)
	t.Setenv("FAKE_REX_LOG", filepath.Join(t.TempDir(), "rex.log"))
	useFakeRex(t, strings.Replace(fakeRexLabelled, `*"session.select"*) ;;`, `*"session.select"*) [ -z "$REFUSE" ] || { echo refused >&2; exit 1; } ;;`, 1))
	configureFakeSources(t, "")
	cfgPath := writeConfig(t, "version = 1\n[history]\nlast_workspace_label = 'prior'\n")
	show := func(current string) {
		rows := `{"sessions":[{"session_id":"a","label":"A"},{"session_id":"b","label":"B"},{"session_id":"c","label":"C"}]}`
		t.Setenv("FAKE_REX_SESSIONS", strings.Replace(rows, `"label":"`+current+`"`, `"label":"`+current+`","current":true`, 1))
	}
	a := &App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	last := func() error { return a.Run(t.Context(), []string{"last", "--opener", "client:mac"}) }
	show("A")
	require.ErrorContains(t, last(), "no previous open session")
	useFakeFZF(t, "\tC\t")
	require.NoError(t, a.Run(t.Context(), []string{"picker", "--fzf", "--config", cfgPath, "--opener", "client:mac"}))
	show("C")
	t.Setenv("REFUSE", "1")
	require.ErrorContains(t, last(), "refused")
	assert.Equal(t, []state.Visit{localVisit("c"), localVisit("a")}, historyOf(t, "id:client:mac"))
	t.Setenv("REFUSE", "")
	require.NoError(t, last())
	show("A")
	require.NoError(t, last())
	show("C")
	log := selectCalls(t, os.Getenv("FAKE_REX_LOG"))
	require.Len(t, log, 4)
	for i, id := range []string{"c", "a", "a", "c"} {
		assert.Contains(t, log[i], "session_id="+id)
	}
	cfg, _, err := config.Load(config.LoadOptions{Path: cfgPath})
	require.NoError(t, err)
	target := switchTarget{opener: "client:mac", currentSessionID: "c", app: rex.AppClient{ID: "client:mac"}}
	b := &pickerBackend{app: a, ctx: t.Context(), cfg: cfg, target: target}
	col, err := b.collect(t.Context(), cfg)
	require.NoError(t, err)
	opts := b.withHistory(b.displayOptions(cfg), col)
	require.NotNil(t, opts.LastWorkspaceLabel)
	assert.Equal(t, "prior", *opts.LastWorkspaceLabel)
	assert.False(t, opts.HideLastWorkspace)
	assert.Equal(t, "a", opts.LastSessionID)
	saved := writeConfig(t, "version = 1\n[picker]\nshow_last_workspace = false\nshow_last_workspace_path = false\n[history]\nlast_workspace_label = '↩'\n")
	opts, result, err := b.ReloadSettings(t.Context(), settings.Result{Path: saved})
	require.NoError(t, err)
	assert.True(t, opts.HideLastWorkspace)
	assert.True(t, opts.HideLastWorkspacePath)
	require.NotNil(t, opts.LastWorkspaceLabel)
	assert.Equal(t, "↩", *opts.LastWorkspaceLabel)
	assert.Equal(t, "a", result.LastSessionID)
}

func TestLastResolvesRemoteVisitAndSkipsClosedSessions(t *testing.T) {
	useHistoryDir(t)
	t.Setenv("FAKE_REX_LOG", filepath.Join(t.TempDir(), "rex.log"))
	script := strings.ReplaceAll(fakeRexHosts, `"kind":"app"`, `"kind":"app","label":"Mac"`)
	useFakeRex(t, script)
	key := "id:client:app"
	remote := state.Visit{Host: "devbox", SessionID: "session:9"}
	recordSwitches(t, key, [2]state.Visit{remote, localVisit("closed")}, [2]state.Visit{localVisit("closed"), localVisit("session:1")})
	last := func() error { return (&App{}).Run(t.Context(), []string{"last", "--client", "client:app"}) }
	require.NoError(t, last())
	assert.Equal(t, []string{"--autostart=false -S devbox -C client:devbox-app do session.select session_id=session:9"}, selectCalls(t, os.Getenv("FAKE_REX_LOG")))
	unavailable := state.Visit{Host: "away", SessionID: "session:unknown"}
	recordSwitches(t, key, [2]state.Visit{unavailable, localVisit("session:1")})
	require.ErrorContains(t, last(), `host "away" did not list`)
	assert.Len(t, selectCalls(t, os.Getenv("FAKE_REX_LOG")), 1)
}

func TestOverlappingLastActionsToggleFromTheLatestAcceptedVisit(t *testing.T) {
	useHistoryDir(t)
	dir := t.TempDir()
	log, release := filepath.Join(dir, "rex.log"), filepath.Join(dir, "release")
	t.Setenv("FAKE_REX_LOG", log)
	t.Setenv("RELEASE", release)
	t.Setenv("FAKE_REX_SESSIONS", `{"sessions":[{"session_id":"a","label":"A"},{"session_id":"c","label":"C","current":true}]}`)
	useFakeRex(t, strings.Replace(fakeRexLabelled, `*"session.select"*) ;;`, `*"session.select"*"session_id=a"*) while [ ! -f "$RELEASE" ]; do sleep 0.01; done ;;`, 1))
	key := "id:client:mac"
	recordSwitches(t, key, [2]state.Visit{localVisit("a"), localVisit("c")})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	results := make(chan error, 2)
	last := func() { results <- (&App{}).Run(ctx, []string{"last", "--opener", "client:mac"}) }
	go last()
	require.Eventually(t, func() bool { return len(callsMatching(t, log, "session.select")) == 1 }, 5*time.Second, 20*time.Millisecond)
	go last()
	// Both commands observed C and resolved their opener before A is recorded.
	require.Eventually(t, func() bool {
		return len(callsMatching(t, log, "client ls")) == 2
	}, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, os.WriteFile(release, nil, 0o600))
	require.NoError(t, <-results)
	require.NoError(t, <-results)
	assert.Equal(t, []string{
		"--autostart=false -C client:mac do session.select session_id=a",
		"--autostart=false -C client:mac do session.select session_id=c",
	}, selectCalls(t, log))
	assert.Equal(t, []state.Visit{localVisit("c"), localVisit("a")}, historyOf(t, key))
}

func TestLastRefusesAnotherAppOnThePreviousHost(t *testing.T) {
	useHistoryDir(t)
	log := filepath.Join(t.TempDir(), "rex.log")
	t.Setenv("FAKE_REX_LOG", log)
	script := strings.NewReplacer(
		`"client_id":"client:app","info":{"kind":"app"}`, `"client_id":"client:app","info":{"kind":"app","label":"Mac"}`,
		`"client_id":"client:devbox-app","info":{"kind":"app"}`, `"client_id":"client:devbox-app","info":{"kind":"app","label":"Tablet"}`,
	).Replace(fakeRexHosts)
	useFakeRex(t, script)
	key := "id:client:app"
	remote := state.Visit{Host: "devbox", SessionID: "session:9"}
	recordSwitches(t, key, [2]state.Visit{remote, localVisit("session:1")})
	err := (&App{}).Run(t.Context(), []string{"last", "--client", "client:app"})
	require.ErrorContains(t, err, `cannot identify this app on host "devbox"`)
	assert.Empty(t, selectCalls(t, log))
	assert.Equal(t, []state.Visit{localVisit("session:1"), remote}, historyOf(t, key))
}

func TestLastUsesHistoryCapturedBeforeEitherChildStarts(t *testing.T) {
	useHistoryDir(t)
	log := filepath.Join(t.TempDir(), "rex.log")
	t.Setenv("FAKE_REX_LOG", log)
	t.Setenv("FAKE_REX_SESSIONS", `{"sessions":[{"session_id":"a"},{"session_id":"b"},{"session_id":"c"}]}`)
	useFakeRex(t, fakeRexLabelled)
	key := "id:client:mac"
	recordSwitches(t, key, [2]state.Visit{localVisit("a"), localVisit("c")})
	snapshot := func() []byte {
		t.Helper()
		data, err := os.ReadFile(state.HistoryPath(stateDir()))
		require.NoError(t, err)
		return data
	}
	last := func(shown string, captured []byte) error {
		t.Helper()
		a := &App{In: bytes.NewReader(captured)}
		return a.Run(t.Context(), []string{"last", "--opener", "client:mac", "--opener-session", shown, "--opener-history-stdin"})
	}
	captured := snapshot()
	require.NoError(t, last("c", captured))
	require.NoError(t, last("c", captured))
	// A native Rex switch to B leaves history untouched. Its current context
	// must still win, even when another app has recorded a switch since.
	manual := snapshot()
	recordSwitches(t, "id:other", [2]state.Visit{localVisit("b"), localVisit("a")})
	require.NoError(t, last("b", manual))
	require.ErrorContains(t, last("b", []byte("{}invalid")), "read captured session history")
	assert.Equal(t, []string{
		"--autostart=false -C client:mac do session.select session_id=a",
		"--autostart=false -C client:mac do session.select session_id=c",
		"--autostart=false -C client:mac do session.select session_id=c",
	}, selectCalls(t, log))
	assert.Equal(t, []state.Visit{localVisit("c"), localVisit("b"), localVisit("a")}, historyOf(t, key))
}
