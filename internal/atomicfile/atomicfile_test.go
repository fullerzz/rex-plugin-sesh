package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceOverwritesTheFile(t *testing.T) {
	dir := t.TempDir()
	path, tmp := filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.tmp")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(tmp, []byte("new"), 0o600))
	require.NoError(t, Replace(tmp, path, nil))
	data, err := os.ReadFile(path) //nolint:gosec // under t.TempDir().
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))
	assert.NoFileExists(t, tmp)
}

// A reader holding the file open delays the replace on Windows; it must not
// fail it.
func TestReplaceWaitsForAReaderToClose(t *testing.T) {
	dir := t.TempDir()
	path, tmp := filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.tmp")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(tmp, []byte("new"), 0o600))
	reader, err := os.Open(path) //nolint:gosec // under t.TempDir().
	require.NoError(t, err)
	closed := make(chan struct{})
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = reader.Close()
		close(closed)
	}()
	require.NoError(t, Replace(tmp, path, nil))
	<-closed
	data, err := os.ReadFile(path) //nolint:gosec // under t.TempDir().
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))
}

func TestReplaceStopsWhenCheckFails(t *testing.T) {
	dir := t.TempDir()
	path, tmp := filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.tmp")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(tmp, []byte("new"), 0o600))
	changed := errors.New("changed on disk")
	require.ErrorIs(t, Replace(tmp, path, func() error { return changed }), changed)
	data, err := os.ReadFile(path) //nolint:gosec // under t.TempDir().
	require.NoError(t, err)
	assert.Equal(t, "old", string(data))
}
