//go:build windows

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An editor holding the config open blocks the rename. If it changes the
// file before it closes it, the save must not overwrite that edit.
func TestWindowsSettingsSaveKeepsAnEditMadeDuringTheReplaceRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte("version=1\n"), 0o600))
	doc, err := OpenSettings(LoadOptions{Path: path})
	require.NoError(t, err)

	editor, err := os.OpenFile(path, os.O_RDWR, 0) //nolint:gosec // Test-owned file.
	require.NoError(t, err)
	saved := make(chan error, 1)
	go func() { saved <- doc.Save(map[string]any{"picker.show_icons": true}) }()
	time.Sleep(200 * time.Millisecond)
	edit := []byte("version=1\n# edited\n")
	_, err = editor.WriteAt(edit, 0)
	require.NoError(t, err)
	require.NoError(t, editor.Close())

	select {
	case err = <-saved:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "Save did not return")
	}
	require.ErrorIs(t, err, ErrSettingsConflict)
	data, err := os.ReadFile(path) //nolint:gosec // Test-owned file.
	require.NoError(t, err)
	assert.Equal(t, edit, data)
}

// A forced migration that waits on a held destination must notice an import
// edited meanwhile, and leave the existing native config alone.
func TestWindowsForcedMigrationRechecksSourcesDuringTheReplaceRetry(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "sesh.toml")
	require.NoError(t, os.WriteFile(source, []byte("dir_length=2\n"), 0o600))
	_, target, err := Migrate(LoadOptions{Path: source}, dir, false)
	require.NoError(t, err)
	existing, err := os.ReadFile(target) //nolint:gosec // Test-owned file.
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(source, []byte("dir_length=3\n"), 0o600))
	migration, err := prepareMigration(LoadOptions{Path: source}, dir, true)
	require.NoError(t, err)
	holder, err := os.OpenFile(target, os.O_RDWR, 0) //nolint:gosec // Test-owned file.
	require.NoError(t, err)
	saved := make(chan error, 1)
	go func() { saved <- migration.Save() }()
	time.Sleep(200 * time.Millisecond)
	require.NoError(t, os.WriteFile(source, []byte("dir_length=4\n"), 0o600))
	require.NoError(t, holder.Close())

	select {
	case err = <-saved:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "Save did not return")
	}
	require.ErrorIs(t, err, ErrSettingsConflict)
	data, err := os.ReadFile(target) //nolint:gosec // Test-owned file.
	require.NoError(t, err)
	assert.Equal(t, existing, data)
}
