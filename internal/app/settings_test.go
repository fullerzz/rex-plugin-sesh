package app

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/fullerzz/rex-plugin-sesh/internal/state"
	"github.com/stretchr/testify/require"
)

func TestConfigEditHelpDoesNotRequireHerdr(t *testing.T) {
	a := New()
	require.ErrorIs(t, a.Run(context.Background(), []string{"config", "edit", "--help"}), flag.ErrHelp)
}

func TestSettingsSaveInvalidatesSessionCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REX_SESH_STATE_DIR", dir)
	path := state.SessionCachePath(dir)
	require.NoError(t, os.WriteFile(path, []byte("cached"), 0600))
	a := New()
	require.NoError(t, a.settingsSaved("config.toml"))
	_, err := os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, a.settingsSaved("config.toml"), "missing cache is already invalidated")
}
