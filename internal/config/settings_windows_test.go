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
