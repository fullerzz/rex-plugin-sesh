package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/fullerzz/rex-plugin-sesh/internal/rex"
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
	assert.Contains(t, string(calls), "--autostart=false do - --json")
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
	assert.Equal(t, "warning: rex sessions unavailable: rex do - --json: no Rex server is running at /tmp/rex.sock, and autostart is disabled\n", errb.String())
}

// A herdr entry in source_order, from a herdr-sesh config, places Rex sessions.
func TestListTreatsHerdrSourceOrderAsRex(t *testing.T) {
	useFakeRex(t, `printf '%s' '{"sessions":[{"session_id":"session:1","label":"api"}]}'`)
	cfgPath := writeConfig(t, "version = 1\n[list]\nsource_order = [\"config\", \"herdr\"]\n[[workspace]]\nname = \"web\"\npath = \"/src/web\"\n")
	assert.Equal(t, []string{"web", "api"}, sessionNames(runListJSON(t, cfgPath, "")))
}

type failingRex struct{}

func (failingRex) Sessions(context.Context) ([]rex.Session, error) {
	return nil, errors.New("no server")
}

// Reloads happen while the picker owns the screen, so their warnings wait.
func TestPickerBackendHoldsRexWarningsUntilFlush(t *testing.T) {
	configureFakeSources(t, "")
	var errb bytes.Buffer
	a := &App{Out: &bytes.Buffer{}, Err: &errb, Rex: failingRex{}}
	backend := &pickerBackend{app: a, ctx: t.Context(), cfg: config.Default()}
	_, err := backend.ReloadPicker(t.Context())
	require.NoError(t, err)
	assert.Empty(t, errb.String())
	backend.flushWarnings()
	assert.Equal(t, "warning: rex sessions unavailable: no server\n", errb.String())
}
