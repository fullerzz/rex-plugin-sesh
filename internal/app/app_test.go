package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCommand(t *testing.T) {
	var out bytes.Buffer
	a := &App{Out: &out, Err: &bytes.Buffer{}}
	require.NoError(t, a.Run(context.Background(), []string{"--version"}))
	assert.Equal(t, "rex-sesh dev", strings.TrimSpace(out.String()))
}

func TestConfigPathCommand(t *testing.T) {
	d := t.TempDir()
	t.Setenv("REX_SESH_CONFIG_DIR", d)
	t.Setenv("REX_SESH_CONFIG", "")
	t.Setenv("HOME", d)

	var out bytes.Buffer
	a := &App{Out: &out, Err: &bytes.Buffer{}}
	require.NoError(t, a.Run(context.Background(), []string{"config", "path"}))
	want := filepath.Join(d, "config.toml")
	require.Equal(t, want, strings.TrimSpace(out.String()))

	legacy := filepath.Join(d, "sesh.toml")
	require.NoError(t, os.WriteFile(legacy, []byte("cache = true\n"), 0600))
	out.Reset()
	require.NoError(t, a.Run(context.Background(), []string{"config", "path"}))
	assert.Equal(t, legacy, strings.TrimSpace(out.String()))
}

func TestConfigValidatePrintsResolvedNativePath(t *testing.T) {
	d := t.TempDir()
	t.Setenv("REX_SESH_CONFIG_DIR", d)
	t.Setenv("REX_SESH_CONFIG", "")
	t.Setenv("HOME", d)
	native := filepath.Join(d, config.NativeFileName)
	require.NoError(t, os.WriteFile(native, []byte("version = 1\n"), 0600))

	var out bytes.Buffer
	a := &App{Out: &out, Err: &bytes.Buffer{}}
	require.NoError(t, a.Run(context.Background(), []string{"config", "validate"}))
	assert.Equal(t, native, strings.TrimSpace(out.String()))
}

func TestConfigValidateWarnsForLegacyConfig(t *testing.T) {
	legacy := filepath.Join(t.TempDir(), config.LegacyFileName)
	require.NoError(t, os.WriteFile(legacy, []byte("cache = true\n"), 0600))

	var out, errb bytes.Buffer
	a := &App{Out: &out, Err: &errb}
	require.NoError(t, a.Run(context.Background(), []string{"config", "validate", legacy}))
	require.Equal(t, legacy, strings.TrimSpace(out.String()))
	assert.Contains(t, errb.String(), "deprecated Sesh-compatible schema")
}

func TestConfigValidateRejectsUnknownLegacyKey(t *testing.T) {
	legacy := filepath.Join(t.TempDir(), config.LegacyFileName)
	require.NoError(t, os.WriteFile(legacy, []byte("cahe = true\n"), 0600))

	a := &App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := a.Run(context.Background(), []string{"config", "validate", legacy})
	require.ErrorContains(t, err, "cahe")
}

func TestConfigValidateRejectsUnknownLegacyKeyInImport(t *testing.T) {
	d := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(d, "extra.toml"), []byte("cahe = true\n"), 0600))
	legacy := filepath.Join(d, config.LegacyFileName)
	require.NoError(t, os.WriteFile(legacy, []byte("import = [\"extra.toml\"]\n"), 0600))

	a := &App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := a.Run(context.Background(), []string{"config", "validate", legacy})
	require.ErrorContains(t, err, "cahe")
}

func TestConfigValidateRejectsInvalidNativeConfig(t *testing.T) {
	native := filepath.Join(t.TempDir(), config.NativeFileName)
	require.NoError(t, os.WriteFile(native, []byte("version = 1\nunknown = true\n"), 0600))

	a := &App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := a.Run(context.Background(), []string{"config", "validate", native})
	require.ErrorContains(t, err, "unknown keys")
}

func TestConfigValidateRequiresExistingConfig(t *testing.T) {
	d := t.TempDir()
	t.Setenv("REX_SESH_CONFIG_DIR", d)
	t.Setenv("REX_SESH_CONFIG", "")
	t.Setenv("HOME", d)

	a := &App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := a.Run(context.Background(), []string{"config", "validate"})
	require.ErrorContains(t, err, "no config file found")
}

func TestConfigValidateAcceptsAtMostOnePath(t *testing.T) {
	a := &App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := a.Run(context.Background(), []string{"config", "validate", "first.toml", "second.toml"})
	require.ErrorContains(t, err, "accepts at most one path")
}

func TestConfigValidateRejectsEmptyPath(t *testing.T) {
	d := t.TempDir()
	t.Setenv("REX_SESH_CONFIG_DIR", d)
	t.Setenv("REX_SESH_CONFIG", "")
	t.Setenv("HOME", d)

	a := &App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := a.Run(context.Background(), []string{"config", "validate", ""})
	require.ErrorContains(t, err, "path must not be empty")
}

func TestConfigInitDoesNotShadowActiveLegacyConfig(t *testing.T) {
	d := t.TempDir()
	t.Setenv("REX_SESH_CONFIG_DIR", d)
	t.Setenv("REX_SESH_CONFIG", "")
	t.Setenv("HOME", d)
	legacy := filepath.Join(d, "sesh.toml")
	require.NoError(t, os.WriteFile(legacy, []byte("cache = true\n"), 0600))

	var out bytes.Buffer
	a := &App{Out: &out, Err: &bytes.Buffer{}}
	require.NoError(t, a.Run(context.Background(), []string{"config", "init"}))
	require.Equal(t, legacy, strings.TrimSpace(out.String()))
	_, err := os.Stat(filepath.Join(d, "config.toml"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestConfigInitHonorsEnvTarget(t *testing.T) {
	d := t.TempDir()
	t.Setenv("REX_SESH_CONFIG_DIR", d)
	t.Setenv("HOME", d)
	target := filepath.Join(d, "custom", "mine.toml")
	t.Setenv("REX_SESH_CONFIG", target)

	var out bytes.Buffer
	a := &App{Out: &out, Err: &bytes.Buffer{}}
	require.NoError(t, a.Run(context.Background(), []string{"config", "init"}))
	require.Equal(t, target, strings.TrimSpace(out.String()))
	cfg, _, err := config.Load(config.LoadOptions{Path: target, Warn: &bytes.Buffer{}})
	require.NoError(t, err)
	assert.Equal(t, config.DefaultPreviewCommand, cfg.DefaultSessionConfig.PreviewCommand)
}

func TestConfigInitRejectsDirectoryCandidate(t *testing.T) {
	d := t.TempDir()
	t.Setenv("REX_SESH_CONFIG_DIR", d)
	t.Setenv("REX_SESH_CONFIG", "")
	t.Setenv("HOME", d)
	require.NoError(t, os.Mkdir(filepath.Join(d, config.NativeFileName), 0700))

	a := &App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := a.Run(context.Background(), []string{"config", "init"})
	require.ErrorContains(t, err, "not a regular file")
}

func TestConfigInitPropagatesResolveError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("REX_SESH_CONFIG_DIR", "")
	t.Setenv("REX_SESH_CONFIG", "")
	t.Setenv("HOME", home)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".config", "sesh"), []byte("blocked\n"), 0600))

	a := &App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := a.Run(context.Background(), []string{"config", "init"})
	require.ErrorIs(t, err, syscall.ENOTDIR)
	_, statErr := os.Stat(filepath.Join(home, ".config", "rex-sesh", config.NativeFileName))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestConfigMigrateCommand(t *testing.T) {
	d := t.TempDir()
	t.Setenv("REX_SESH_CONFIG_DIR", d)
	t.Setenv("REX_SESH_CONFIG", "")
	t.Setenv("HOME", d)
	legacy := filepath.Join(d, "sesh.toml")
	require.NoError(t, os.WriteFile(legacy, []byte("cache = true\n[[session]]\nname = \"api\"\npath = \"/tmp/api\"\n"), 0600))

	var out, errb bytes.Buffer
	a := &App{Out: &out, Err: &errb}
	require.NoError(t, a.Run(context.Background(), []string{"config", "migrate"}))
	native := filepath.Join(d, "config.toml")
	require.Equal(t, native, strings.TrimSpace(out.String()))
	require.Contains(t, errb.String(), "delete the legacy file")
	cfg, _, err := config.Load(config.LoadOptions{Path: native, Warn: &bytes.Buffer{}})
	require.NoError(t, err)
	require.True(t, cfg.Cache)
	require.Len(t, cfg.SessionConfigs, 1)
	require.Equal(t, "api", cfg.SessionConfigs[0].Name)
	// The native file now wins discovery over the untouched legacy file.
	got, err := config.ResolvePath(config.LoadOptions{})
	require.NoError(t, err)
	assert.Equal(t, native, got)
}

func TestConfigMigrateForceOverwritesExistingNativeConfig(t *testing.T) {
	d := t.TempDir()
	t.Setenv("REX_SESH_CONFIG_DIR", d)
	t.Setenv("REX_SESH_CONFIG", "")
	t.Setenv("HOME", d)
	legacy := filepath.Join(d, "sesh.toml")
	require.NoError(t, os.WriteFile(legacy, []byte("cache = true\n"), 0600))
	native := filepath.Join(d, config.NativeFileName)
	require.NoError(t, os.WriteFile(native, []byte("version = 1\n[list]\ncache = false\n"), 0600))

	var out bytes.Buffer
	a := &App{Out: &out, Err: &bytes.Buffer{}}
	require.NoError(t, a.Run(context.Background(), []string{"config", "migrate", legacy, "--force"}))
	require.Equal(t, native, strings.TrimSpace(out.String()))
	cfg, _, err := config.Load(config.LoadOptions{Path: native, Warn: &bytes.Buffer{}})
	require.NoError(t, err)
	assert.True(t, cfg.Cache, "forced migration did not replace existing native config")
}

func TestConfigMigrateWarnsToRepointEnvBeforeDeletingLegacy(t *testing.T) {
	d := t.TempDir()
	legacy := filepath.Join(d, config.LegacyFileName)
	native := filepath.Join(d, config.NativeFileName)
	t.Setenv("REX_SESH_CONFIG_DIR", d)
	t.Setenv("REX_SESH_CONFIG", legacy)
	t.Setenv("HOME", d)
	require.NoError(t, os.WriteFile(legacy, []byte("cache = true\n"), 0600))

	var errb bytes.Buffer
	a := &App{Out: &bytes.Buffer{}, Err: &errb}
	require.NoError(t, a.Run(context.Background(), []string{"config", "migrate"}))
	warning := errb.String()
	assert.Contains(t, warning, "set REX_SESH_CONFIG="+native+" before deleting "+legacy)
}

func TestConfigMigrateKeepsUnrelatedEnvSelection(t *testing.T) {
	d := t.TempDir()
	active := filepath.Join(d, "active", config.LegacyFileName)
	legacy := filepath.Join(d, "other", config.LegacyFileName)
	t.Setenv("REX_SESH_CONFIG_DIR", d)
	t.Setenv("REX_SESH_CONFIG", active)
	t.Setenv("HOME", d)
	for _, p := range []string{active, legacy} {
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0700))
		require.NoError(t, os.WriteFile(p, []byte("cache = true\n"), 0600))
	}

	var errb bytes.Buffer
	a := &App{Out: &bytes.Buffer{}, Err: &errb}
	require.NoError(t, a.Run(context.Background(), []string{"config", "migrate", "--config", legacy}))
	warning := errb.String()
	require.NotContains(t, warning, "set REX_SESH_CONFIG=")
	assert.Contains(t, warning, "REX_SESH_CONFIG continues to select "+active)
}

func TestConfigMigrateExplicitPathWarnsWhenNativeIsNotDiscovered(t *testing.T) {
	home := t.TempDir()
	legacyDir := t.TempDir()
	legacy := filepath.Join(legacyDir, config.LegacyFileName)
	native := filepath.Join(legacyDir, config.NativeFileName)
	t.Setenv("REX_SESH_CONFIG_DIR", filepath.Join(home, "plugin-config"))
	t.Setenv("REX_SESH_CONFIG", "")
	t.Setenv("HOME", home)
	require.NoError(t, os.WriteFile(legacy, []byte("cache = true\n"), 0600))

	var errb bytes.Buffer
	a := &App{Out: &bytes.Buffer{}, Err: &errb}
	require.NoError(t, a.Run(context.Background(), []string{"config", "migrate", legacy}))
	warning := errb.String()
	require.NotContains(t, warning, "now takes precedence")
	assert.Contains(t, warning, "set REX_SESH_CONFIG="+native+" before deleting "+legacy)
}

func TestConfigMigrateLegacySymlinkDoesNotClaimNativePrecedence(t *testing.T) {
	home := t.TempDir()
	pluginDir := filepath.Join(home, "plugin-config")
	legacyDir := t.TempDir()
	legacy := filepath.Join(legacyDir, config.LegacyFileName)
	native := filepath.Join(legacyDir, config.NativeFileName)
	legacyLink := filepath.Join(pluginDir, config.LegacyFileName)
	t.Setenv("REX_SESH_CONFIG_DIR", pluginDir)
	t.Setenv("REX_SESH_CONFIG", "")
	t.Setenv("HOME", home)
	require.NoError(t, os.MkdirAll(pluginDir, 0700))
	require.NoError(t, os.WriteFile(legacy, []byte("cache = true\n"), 0600))
	require.NoError(t, os.Symlink(native, legacyLink))

	var errb bytes.Buffer
	a := &App{Out: &bytes.Buffer{}, Err: &errb}
	require.NoError(t, a.Run(context.Background(), []string{"config", "migrate", legacy}))
	cfg, resolved, err := config.Load(config.LoadOptions{Warn: &bytes.Buffer{}})
	require.NoError(t, err)
	require.Equal(t, legacyLink, resolved)
	require.False(t, cfg.Cache)
	warning := errb.String()
	require.NotContains(t, warning, "now takes precedence")
	assert.Contains(t, warning, "set REX_SESH_CONFIG="+native)
}

func TestListIgnoresCorruptSessionCache(t *testing.T) {
	d := t.TempDir()
	cfgPath := filepath.Join(d, "sesh.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("cache = true\n[[session]]\nname = \"api\"\npath = \"/tmp/api\"\n"), 0600))
	stateDir := filepath.Join(d, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "sessions.json"), []byte("{"), 0600))
	t.Setenv("REX_SESH_STATE_DIR", stateDir)

	var out, errb bytes.Buffer
	a := &App{Out: &out, Err: &errb}
	require.NoError(t, a.Run(context.Background(), []string{"list", "--json", "--config", cfgPath}))
	require.Contains(t, out.String(), `"name": "api"`)
	assert.Contains(t, errb.String(), "warning: ignoring session cache")
}

func TestListWarnsWhenSessionCacheCannotBeSaved(t *testing.T) {
	d := t.TempDir()
	cfgPath := filepath.Join(d, "sesh.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("cache = true\n[[session]]\nname = \"api\"\npath = \"/tmp/api\"\n"), 0600))
	statePath := filepath.Join(d, "state-file")
	require.NoError(t, os.WriteFile(statePath, []byte("not a directory"), 0600))
	t.Setenv("REX_SESH_STATE_DIR", statePath)

	var out, errb bytes.Buffer
	a := &App{Out: &out, Err: &errb}
	require.NoError(t, a.Run(context.Background(), []string{"list", "--json", "--config", cfgPath}))
	require.Contains(t, out.String(), `"name": "api"`)
	require.Contains(t, errb.String(), "warning: ignoring session cache")
	assert.Contains(t, errb.String(), "warning: could not save session cache")
}

func TestListCacheDoesNotMaskBlacklistedResults(t *testing.T) {
	d := t.TempDir()
	cfgPath := filepath.Join(d, "sesh.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`cache = true
blacklist = ["^scratch$"]

[[session]]
name = "api"
path = "/tmp/api"

[[session]]
name = "scratch"
path = "/tmp/scratch"
`), 0600))
	t.Setenv("REX_SESH_STATE_DIR", filepath.Join(d, "state"))

	got := runListJSON(t, cfgPath, "")
	require.Len(t, got, 1)
	assert.Equal(t, "api", got[0].Name)
	got = runListJSON(t, cfgPath, "", "--blacklisted")
	require.Len(t, got, 1)
	assert.Equal(t, "scratch", got[0].Name)
}

func TestListCacheDoesNotMaskDuplicateResults(t *testing.T) {
	d := t.TempDir()
	cfgPath := filepath.Join(d, "sesh.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`cache = true
sort_order = ["config", "zoxide"]

[[session]]
name = "api"
path = "/configured/api"
`), 0600))
	t.Setenv("REX_SESH_STATE_DIR", filepath.Join(d, "state"))
	zoxideOutput := "42 /discovered/api\n"

	require.Len(t, runListJSON(t, cfgPath, zoxideOutput), 1)
	require.Len(t, runListJSON(t, cfgPath, zoxideOutput, "--hide-duplicates=false"), 2)
}

func TestListCacheDoesNotCrossConfigFiles(t *testing.T) {
	d := t.TempDir()
	firstConfig := filepath.Join(d, "first.toml")
	secondConfig := filepath.Join(d, "second.toml")
	require.NoError(t, os.WriteFile(firstConfig, []byte("cache = true\n[[session]]\nname = \"api\"\npath = \"/tmp/api\"\n"), 0600))
	require.NoError(t, os.WriteFile(secondConfig, []byte("cache = true\n[[session]]\nname = \"web\"\npath = \"/tmp/web\"\n"), 0600))
	t.Setenv("REX_SESH_STATE_DIR", filepath.Join(d, "state"))

	got := runListJSON(t, firstConfig, "")
	require.Len(t, got, 1)
	assert.Equal(t, "api", got[0].Name)
	got = runListJSON(t, secondConfig, "")
	require.Len(t, got, 1)
	assert.Equal(t, "web", got[0].Name)
}

func TestListCacheDistinguishesRelativeConfigsAcrossWorkingDirectories(t *testing.T) {
	d := t.TempDir()
	firstDir := filepath.Join(d, "first")
	secondDir := filepath.Join(d, "second")
	for dir, name := range map[string]string{firstDir: "api", secondDir: "web"} {
		require.NoError(t, os.Mkdir(dir, 0700))
		body := fmt.Sprintf("cache = true\n[[session]]\nname = %q\npath = %q\n", name, filepath.Join("/tmp", name))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "sesh.toml"), []byte(body), 0600))
	}
	t.Setenv("REX_SESH_STATE_DIR", filepath.Join(d, "state"))

	t.Chdir(firstDir)
	got := runListJSON(t, "sesh.toml", "")
	require.Len(t, got, 1)
	assert.Equal(t, "api", got[0].Name)
	t.Chdir(secondDir)
	got = runListJSON(t, "sesh.toml", "")
	require.Len(t, got, 1)
	assert.Equal(t, "web", got[0].Name)
}

func TestPickerOptionsPropagateWorkspaceSort(t *testing.T) {
	cfg := config.Default()
	require.Equal(t, "workspace", pickerOptionsFromConfig(context.Background(), io.Discard, cfg).WorkspaceSort)
	cfg.TUI.DefaultSort = "agent"
	assert.Equal(t, "agent", pickerOptionsFromConfig(context.Background(), io.Discard, cfg).WorkspaceSort)
}

func TestPickerOptionsPropagateWorktreeIconReplacement(t *testing.T) {
	cfg := config.Default()
	require.False(t, pickerOptionsFromConfig(context.Background(), io.Discard, cfg).DisableWorktreeIconReplacement, "default config disabled worktree icon replacement")
	cfg.TUI.ReplaceWorktreeIcon = false
	assert.True(t, pickerOptionsFromConfig(context.Background(), io.Discard, cfg).DisableWorktreeIconReplacement, "disabled worktree icon replacement was not propagated")
}

func TestPickerOptionsPropagatePreviewVisibility(t *testing.T) {
	cfg := config.Default()
	require.False(t, pickerOptionsFromConfig(context.Background(), io.Discard, cfg).HidePreview, "default config hid native picker preview")
	cfg.TUI.ShowPreview = false
	assert.True(t, pickerOptionsFromConfig(context.Background(), io.Discard, cfg).HidePreview, "show_preview=false was not propagated")
}

func TestPickerOptionsPropagatePreviewMode(t *testing.T) {
	cfg := config.Default()
	require.Equal(t, "command", pickerOptionsFromConfig(context.Background(), io.Discard, cfg).PreviewMode)
	cfg.TUI.PreviewMode = "pane"
	assert.Equal(t, "pane", pickerOptionsFromConfig(context.Background(), io.Discard, cfg).PreviewMode)
}

func TestPickerOptionsPropagateHomePrioritization(t *testing.T) {
	cfg := config.Default()
	require.False(t, pickerOptionsFromConfig(context.Background(), io.Discard, cfg).DisableHomePrioritization, "default config disabled home prioritization")
	cfg.TUI.PrioritizeHome = false
	assert.True(t, pickerOptionsFromConfig(context.Background(), io.Discard, cfg).DisableHomePrioritization, "prioritize_home=false was not propagated")
}

func TestPickerJSONCommand(t *testing.T) {
	var out bytes.Buffer
	a := &App{Out: &out, Err: &bytes.Buffer{}}
	require.NoError(t, a.Run(context.Background(), []string{"picker", "--json", "--config", filepath.Join("..", "..", "testdata", "sesh.toml")}))
	assert.Contains(t, out.String(), `"name": "sesh"`)
}

func TestPickerJSONAppliesDefaultStartupCommand(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "sesh.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`[default_session]
startup_command = "printf default:{}"

[[session]]
name = "api"
path = "/tmp/api"
`), 0600))

	sessions := runPickerJSON(t, cfgPath, "")
	require.Len(t, sessions, 1)
	assert.Equal(t, "printf default:{}", sessions[0].StartupCommand)
}

func TestPickerJSONAppliesWildcardSettings(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	require.NoError(t, os.Mkdir(project, 0700))
	cfgPath := filepath.Join(t.TempDir(), "sesh.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`strict_mode = true

[[wildcard]]
pattern = "`+project+`"
startup_command = "printf wildcard:{}"
preview_command = "printf preview:{}"
disable_startup_command = true
windows = ["git"]

[[window]]
name = "git"
startup_script = "git status"
`), 0600))

	sessions := runPickerJSON(t, cfgPath, "42 "+project+"\n")
	require.Len(t, sessions, 1)
	s := sessions[0]
	require.Empty(t, s.StartupCommand)
	require.Equal(t, "printf preview:{}", s.PreviewCommand)
	require.True(t, s.DisableStartupCommand)
	require.Equal(t, []string{"git"}, s.WindowNames)
	assert.Empty(t, s.WindowConfigs)
}

func TestPickerJSONExplicitFalseOverridesWildcardDisable(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	require.NoError(t, os.Mkdir(project, 0700))
	cfgPath := filepath.Join(t.TempDir(), "sesh.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`[default_session]
startup_command = "printf default:{}"

[[session]]
name = "project"
path = "`+project+`"
disable_startup_command = false

[[wildcard]]
pattern = "`+project+`"
startup_command = "printf wildcard:{}"
disable_startup_command = true
`), 0600))

	sessions := runPickerJSON(t, cfgPath, "")
	require.Len(t, sessions, 1)
	require.False(t, sessions[0].DisableStartupCommand)
	assert.Equal(t, "printf wildcard:{}", sessions[0].StartupCommand)
}

func TestCollectDirectPathUsesConfiguredDirLength(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "parent")
	target := filepath.Join(parent, "child")
	require.NoError(t, os.MkdirAll(target, 0700))
	configureFakeSources(t, "")
	cfg := config.Default()
	cfg.DirLength = 2

	sessions, err := (&App{}).collect(context.Background(), cfg, target, false)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, filepath.Join("parent", "child"), sessions[0].Name)
}

func TestPreviewCommandUsesExplicitConfig(t *testing.T) {
	d := t.TempDir()
	targetDir := filepath.Join(d, "target")
	require.NoError(t, os.Mkdir(targetDir, 0700))
	cfgPath := filepath.Join(d, "sesh.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("[default_session]\npreview_command = \"printf configured:%s {}\"\n"), 0600))
	fakeBin := filepath.Join(d, "bin")
	require.NoError(t, os.MkdirAll(fakeBin, 0700))
	//nolint:gosec // test creates a local executable fixture.
	require.NoError(t, os.WriteFile(filepath.Join(fakeBin, "zoxide"), []byte("#!/bin/sh\nexit 1\n"), 0700))
	//nolint:gosec // test creates a local executable fixture.
	require.NoError(t, os.WriteFile(filepath.Join(fakeBin, "eza"), []byte("#!/bin/sh\nprintf 'default:%s\\n' \"$*\"\n"), 0700))
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	var out bytes.Buffer
	a := &App{Out: &out, Err: &bytes.Buffer{}}
	require.NoError(t, a.Run(context.Background(), []string{"preview", "--config", cfgPath, targetDir}))
	require.Contains(t, out.String(), "configured:")
	assert.Contains(t, out.String(), targetDir)
}

func runPickerJSON(t *testing.T, cfgPath, zoxideOutput string) []model.Session {
	t.Helper()
	configureFakeSources(t, zoxideOutput)

	var out bytes.Buffer
	a := &App{Out: &out, Err: &bytes.Buffer{}}
	require.NoError(t, a.Run(context.Background(), []string{"picker", "--json", "--config", cfgPath}))
	var sessions []model.Session
	require.NoError(t, json.Unmarshal(out.Bytes(), &sessions))
	return sessions
}

func runListJSON(t *testing.T, cfgPath, zoxideOutput string, extraArgs ...string) []model.Session {
	t.Helper()
	configureFakeSources(t, zoxideOutput)

	args := append([]string{"list", "--json", "--config", cfgPath}, extraArgs...)
	var out bytes.Buffer
	a := &App{Out: &out, Err: &bytes.Buffer{}}
	require.NoError(t, a.Run(context.Background(), args))
	var sessions []model.Session
	require.NoError(t, json.Unmarshal(out.Bytes(), &sessions))
	return sessions
}

func configureFakeSources(t *testing.T, zoxideOutput string) {
	t.Helper()
	fakeBin := t.TempDir()
	//nolint:gosec // test creates a local executable fixture.
	require.NoError(t, os.WriteFile(filepath.Join(fakeBin, "zoxide"), []byte("#!/bin/sh\nprintf '%s' \"$FAKE_ZOXIDE_OUTPUT\"\n"), 0700))
	t.Setenv("FAKE_ZOXIDE_OUTPUT", zoxideOutput)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestPickerOptionsPropagateCyclePreviewModeKey(t *testing.T) {
	for _, binding := range []string{"ctrl+o", "alt+p", ""} {
		cfg := config.Default()
		cfg.Keys.CyclePreviewMode = binding
		opts := pickerOptionsFromConfig(context.Background(), io.Discard, cfg)
		require.NotNil(t, opts.CyclePreviewModeKey)
		assert.Equal(t, binding, *opts.CyclePreviewModeKey)
	}
}

func TestPickerOptionsPropagatePathVisibility(t *testing.T) {
	cfg := config.Default()
	require.False(t, pickerOptionsFromConfig(context.Background(), io.Discard, cfg).HidePath)
	cfg.TUI.ShowPath = false
	assert.True(t, pickerOptionsFromConfig(context.Background(), io.Discard, cfg).HidePath)
}
