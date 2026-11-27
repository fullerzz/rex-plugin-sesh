package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/fullerzz/rex-plugin-sesh/internal/rex"
	"github.com/fullerzz/rex-plugin-sesh/internal/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useFakeRex points rex at a script that runs body, for this test only.
func useFakeRex(t *testing.T, body string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "rex")
	//nolint:gosec // the fake rex must be executable.
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"+body+"\n"), 0o700))
	t.Setenv("REX_SESH_REX_BIN", bin)
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestAppTestsReachOnlyTheFakeRex(t *testing.T) {
	runListJSON(t, writeConfig(t, "version = 1\n"), "")
	//nolint:gosec // fakeRexLog is the TestMain fake's log.
	calls, err := os.ReadFile(fakeRexLog)
	require.NoError(t, err)
	assert.Contains(t, string(calls), "--autostart=false do - --args")
}

func TestListPutsRexSessionsFirst(t *testing.T) {
	useFakeRex(t, `printf '%s' '{"sessions":[{"session_id":"session:1","label":"api","windows":1,"pwd":"file://mac/src/api"}]}'`)
	cfgPath := writeConfig(t, "version = 1\n[[workspace]]\nname = \"web\"\npath = \"/src/web\"\n")
	sessions := runListJSON(t, cfgPath, "")
	require.Len(t, sessions, 2)
	assert.Equal(t, model.Session{Source: "rex", Name: "api", Path: "/src/api", WorkspaceID: "session:1"}, sessions[0])
	assert.Equal(t, "web", sessions[1].Name)
}

func TestListWarnsAndContinuesWithoutRex(t *testing.T) {
	useFakeRex(t, `printf '%s\n' 'Error: no Rex server is running at /tmp/rex.sock, and autostart is disabled' >&2; exit 4`)
	configureFakeSources(t, "")
	cfgPath := writeConfig(t, "version = 1\n[[workspace]]\nname = \"web\"\npath = \"/src/web\"\n")
	var out, errb bytes.Buffer
	a := &App{Out: &out, Err: &errb}
	require.NoError(t, a.Run(context.Background(), []string{"list", "--config", cfgPath}))
	assert.Equal(t, "config\tweb\t/src/web\n", out.String())
	assert.Equal(t, "warning: rex sessions unavailable: rex do: no Rex server is running at /tmp/rex.sock, and autostart is disabled\n", errb.String())
}

// A herdr entry in source_order, from a herdr-sesh config, places Rex sessions.
func TestListTreatsHerdrSourceOrderAsRex(t *testing.T) {
	useFakeRex(t, `printf '%s' '{"sessions":[{"session_id":"session:1","label":"api"}]}'`)
	cfgPath := writeConfig(t, "version = 1\n[list]\nsource_order = [\"config\", \"herdr\"]\n[[workspace]]\nname = \"web\"\npath = \"/src/web\"\n")
	assert.Equal(t, []string{"web", "api"}, sessionNames(runListJSON(t, cfgPath, "")))
}

type failingRex struct{}

func (failingRex) Sessions(context.Context, string) ([]rex.Session, error) {
	return nil, errors.New("no server")
}

const twoRexSessions = `printf '%s\n' "$*" >> "$FAKE_REX_LOG"; printf '%s' '{"sessions":[{"session_id":"session:1","label":"api","pwd":"file://mac/src/api"},{"session_id":"session:2","label":"web","current":true}]}'`

func runPickerJSON(t *testing.T, cfgPath string) []model.Session {
	t.Helper()
	var out bytes.Buffer
	a := &App{Out: &out, Err: &bytes.Buffer{}}
	require.NoError(t, a.Run(context.Background(), []string{"picker", "--json", "--config", cfgPath}))
	var sessions []model.Session
	require.NoError(t, json.Unmarshal(out.Bytes(), &sessions))
	return sessions
}

// The picker shows open Rex sessions only, and marks the one that holds this
// terminal's block.
func TestPickerListsOnlyRexSessionsAndMarksCurrent(t *testing.T) {
	log := filepath.Join(t.TempDir(), "rex.log")
	t.Setenv("FAKE_REX_LOG", log)
	useFakeRex(t, twoRexSessions)
	configureFakeSources(t, "1 /src/zoxide-dir\n")
	t.Setenv("REX_BLOCK", "block:42")
	cfgPath := writeConfig(t, "version = 1\n[[workspace]]\nname = \"cfg\"\npath = \"/src/cfg\"\n")
	sessions := runPickerJSON(t, cfgPath)
	assert.Equal(t, []model.Session{
		{Source: "rex", Name: "api", Path: "/src/api", WorkspaceID: "session:1"},
		{Source: "rex", Name: "web", WorkspaceID: "session:2", Attached: true},
	}, sessions)
	//nolint:gosec // log is in this test's temporary directory.
	calls, err := os.ReadFile(log)
	require.NoError(t, err)
	assert.Contains(t, string(calls), `--args {"block_id":"block:42"}`)
}

// list caches its output for every session, so it never marks one current.
func TestListDoesNotMarkCurrentSession(t *testing.T) {
	t.Setenv("FAKE_REX_LOG", filepath.Join(t.TempDir(), "rex.log"))
	useFakeRex(t, twoRexSessions)
	t.Setenv("REX_BLOCK", "block:42")
	runListJSON(t, writeConfig(t, "version = 1\n"), "")
	//nolint:gosec // FAKE_REX_LOG is in this test's temporary directory.
	calls, err := os.ReadFile(os.Getenv("FAKE_REX_LOG"))
	require.NoError(t, err)
	assert.Contains(t, string(calls), `--args {"block_id":""}`)
}

func TestPickerFailsBeforeOpeningWithoutRex(t *testing.T) {
	useFakeRex(t, `printf '%s\n' 'Error: no Rex server is running at /tmp/rex.sock, and autostart is disabled' >&2; exit 4`)
	cfgPath := writeConfig(t, "version = 1\n")
	for _, args := range [][]string{{"picker", "--json"}, {"picker"}, {"picker", "--fzf"}} {
		var out bytes.Buffer
		a := &App{Out: &out, Err: &bytes.Buffer{}}
		err := a.Run(context.Background(), append(args, "--config", cfgPath))
		require.EqualError(t, err, "list Rex sessions: rex do: no Rex server is running at /tmp/rex.sock, and autostart is disabled", args)
		assert.Empty(t, out.String(), args)
	}
}

// Reloads happen while the picker owns the screen: they stay Rex-only and
// return Rex errors for the picker to show, writing nothing to stderr.
func TestPickerReloadsStayRexOnly(t *testing.T) {
	t.Setenv("FAKE_REX_LOG", filepath.Join(t.TempDir(), "rex.log"))
	useFakeRex(t, twoRexSessions)
	configureFakeSources(t, "1 /src/zoxide-dir\n")
	cfgPath := writeConfig(t, "version = 1\n[[workspace]]\nname = \"cfg\"\npath = \"/src/cfg\"\n")
	a := &App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	cfg, err := a.loadConfig(cfgPath)
	require.NoError(t, err)
	backend := &pickerBackend{app: a, ctx: t.Context(), cfg: cfg, activeConfigPath: cfgPath}

	reloaded, err := backend.ReloadPicker(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{"api", "web"}, sessionNames(reloaded.Sessions))
	_, afterSettings, err := backend.ReloadSettings(t.Context(), settings.Result{Path: cfgPath, Saved: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"api", "web"}, sessionNames(afterSettings.Sessions))
}

func TestPickerReloadReportsRexError(t *testing.T) {
	configureFakeSources(t, "")
	var errb bytes.Buffer
	a := &App{Out: &bytes.Buffer{}, Err: &errb, Rex: failingRex{}}
	backend := &pickerBackend{app: a, ctx: t.Context(), cfg: config.Default()}
	_, err := backend.ReloadPicker(t.Context())
	require.EqualError(t, err, "list Rex sessions: no server")
	assert.Empty(t, errb.String())
}
