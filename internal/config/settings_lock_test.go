package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fullerzz/rex-plugin-sesh/internal/filelock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Windows CI job runs the TestSettingsLock and TestWindows tests by name.

func TestSettingsLockMakesASaveFailFastThenSucceedAfterRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "has space", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	original := []byte("version=1\n")
	require.NoError(t, os.WriteFile(path, original, 0o600))
	doc, err := OpenSettings(LoadOptions{Path: path})
	require.NoError(t, err)

	lock, err := filelock.Open(path + ".settings.lock")
	require.NoError(t, err)
	defer func() { _ = lock.Close() }()
	require.NoError(t, filelock.TryLock(lock))
	err = doc.Save(map[string]any{"picker.show_icons": true})
	require.ErrorIs(t, err, filelock.ErrWouldBlock)
	assert.Contains(t, err.Error(), "config is busy")
	data, err := os.ReadFile(path) //nolint:gosec // Test-owned file.
	require.NoError(t, err)
	assert.Equal(t, original, data)

	require.NoError(t, filelock.Unlock(lock))
	require.NoError(t, doc.Save(map[string]any{"picker.show_icons": true}))
	data, err = os.ReadFile(path) //nolint:gosec // Test-owned file.
	require.NoError(t, err)
	assert.Contains(t, string(data), "show_icons = true")
}

func TestSettingsLockRefusesARedirectedLockFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	original := []byte("version=1\n")
	require.NoError(t, os.WriteFile(path, original, 0o600))
	elsewhere := filepath.Join(dir, "elsewhere.lock")
	require.NoError(t, os.WriteFile(elsewhere, nil, 0o600))
	if err := os.Symlink(elsewhere, path+".settings.lock"); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
	doc, err := OpenSettings(LoadOptions{Path: path})
	require.NoError(t, err)
	require.Error(t, doc.Save(map[string]any{"picker.show_icons": true}))
	data, err := os.ReadFile(path) //nolint:gosec // Test-owned file.
	require.NoError(t, err)
	assert.Equal(t, original, data)
}
