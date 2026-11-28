package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const localAndRemoteConfig = `version = 1
[[workspace]]
name = "web"
path = "/src/web"
[[workspace]]
name = "api"
machine = "buntu26"
path = "/srv/api"
`

func sessionNames(sessions []model.Session) []string {
	names := make([]string, 0, len(sessions))
	for _, s := range sessions {
		names = append(names, s.DisplayName())
	}
	return names
}

func TestListTextLabelsRemoteSessionsOnly(t *testing.T) {
	var out bytes.Buffer
	a := &App{Out: &out}
	require.NoError(t, a.printSessions([]model.Session{
		{Source: "config", Name: "api", Path: "/src/api"},
		{Source: "config", Machine: "buntu26", Name: "api", Path: "/src/api"},
		{Source: "config", Machine: "buntu26", Name: "~"},
		{Source: "config", Name: "notes"},
	}, false))
	assert.Equal(t, "config\tapi\t/src/api\nconfig\t[buntu26] api\t/src/api\nconfig\t[buntu26] ~\nconfig\tnotes\n", out.String())
}

func TestCollectForRemoteTargetRunsNoLocalGit(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "git.log")
	//nolint:gosec // the test's fake git must be executable.
	require.NoError(t, os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\necho \"$@\" >> '"+log+"'\nexit 1\n"), 0o755))
	t.Setenv("PATH", bin) // Also excludes the user's zoxide database.
	t.Chdir(t.TempDir())
	cfg := config.Default()
	cfg.SessionConfigs = []config.SessionConfig{{Name: "api", Machine: "buntu26", Path: "/remote-only/api"}}
	for _, target := range []string{"api", "/remote-only/api"} {
		col, err := (&App{}).collect(context.Background(), cfg, target, false)
		sessions := col.Sessions
		require.NoError(t, err)
		require.Len(t, sessions, 1)
		assert.Equal(t, "buntu26", sessions[0].Machine)
	}
	assert.NoFileExists(t, log, "git ran locally for a remote target")

	// The sentinel works: a local directory target is still named with git.
	local := t.TempDir()
	_, err := (&App{}).collect(context.Background(), cfg, local, false)
	require.NoError(t, err)
	assert.FileExists(t, log)
}

// rex-sesh cannot open a [[workspace]] entry on another machine yet, so list
// leaves it out.
func TestListOmitsRemoteWorkspaces(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(localAndRemoteConfig), 0o600))
	assert.Equal(t, []string{"web"}, sessionNames(runListJSON(t, cfgPath, "")))
}

// Preview keeps a remote entry, which shows its metadata without running
// anything.
func TestPreviewKeepsRemoteWorkspaces(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(localAndRemoteConfig), 0o600))
	configureFakeSources(t, "")
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	a := &App{Out: &out, Err: &bytes.Buffer{}}
	require.NoError(t, a.preview(t.Context(), []string{"--config", cfgPath, "api"}))
	assert.Equal(t, "workspace: api\nmachine: buntu26\nid: \npath: /srv/api\n", out.String())
}

func TestPreviewConfiguredRemoteNamePrefersLocalDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	configureFakeSources(t, "")
	cfg := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(cfg, []byte(`version = 1
[workspace_defaults]
preview = "printf local-preview"
[[workspace]]
name = "api"
machine = "buntu26"
path = "/remote-only/api"
`), 0o600))
	var out bytes.Buffer
	a := &App{Out: &out, Err: &bytes.Buffer{}}
	require.NoError(t, a.preview(t.Context(), []string{"--config", cfg, "api"}))
	assert.Contains(t, out.String(), "machine: buntu26")
	require.NoError(t, os.Mkdir("api", 0o700))
	out.Reset()
	require.NoError(t, a.preview(t.Context(), []string{"--config", cfg, "api"}))
	assert.Equal(t, "local-preview", out.String())
}

func TestStateDirPrefersOverrideThenXDGStateHome(t *testing.T) {
	override := t.TempDir()
	t.Setenv("REX_SESH_STATE_DIR", override)
	assert.Equal(t, override, stateDir())

	xdg := t.TempDir()
	t.Setenv("REX_SESH_STATE_DIR", "")
	t.Setenv("XDG_STATE_HOME", xdg)
	assert.Equal(t, filepath.Join(xdg, "rex-sesh"), stateDir())

	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", home)
	assert.Equal(t, filepath.Join(home, ".local", "state", "rex-sesh"), stateDir())
}

func TestLocalPickerCollectionSkipsHostLookup(t *testing.T) {
	log := filepath.Join(t.TempDir(), "rex.log")
	t.Setenv("FAKE_REX_LOG", log)
	useFakeRex(t, fakeRexHosts)
	col, err := (&App{}).collectPickerRows(t.Context(), config.Default(), false)
	require.NoError(t, err)
	require.NoError(t, col.RexErr)
	assert.Equal(t, []string{"api"}, sessionNames(col.RexSessions))
	calls, err := os.ReadFile(log) //nolint:gosec // The test owns this temporary command log.
	require.NoError(t, err)
	assert.NotContains(t, string(calls), "hosts --json")
	assert.NotContains(t, string(calls), "-S ")
}
