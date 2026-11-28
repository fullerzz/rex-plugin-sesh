//go:build windows

package preview

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelperProcess is the descendant the runShell tests start. It writes
// READY_FILE, waits for RELEASE_FILE, then writes SURVIVED_FILE. With
// -- wait it only waits for READY_FILE.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("PREVIEW_HELPER") != "1" {
		t.Skip("helper process only")
	}
	if os.Args[len(os.Args)-1] == "wait" {
		waitFor(os.Getenv("READY_FILE"))
		os.Exit(0)
	}
	//nolint:gosec // the parent test names files under its t.TempDir().
	_ = os.WriteFile(os.Getenv("READY_FILE"), []byte("ready"), 0o600)
	waitFor(os.Getenv("RELEASE_FILE"))
	//nolint:gosec // the parent test names files under its t.TempDir().
	_ = os.WriteFile(os.Getenv("SURVIVED_FILE"), []byte("survived"), 0o600)
	os.Exit(0)
}

func waitFor(path string) {
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		//nolint:gosec // the parent test names files under its t.TempDir().
		if _, err := os.Stat(path); err == nil {
			return
		}
	}
}

type helperFiles struct{ ready, release, survived string }

// helper sets up TestHelperProcess and returns the cmd.exe command that
// starts it, quoted for a test binary path with spaces.
func helper(t *testing.T) (helperFiles, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "has space")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	files := helperFiles{filepath.Join(dir, "ready"), filepath.Join(dir, "release"), filepath.Join(dir, "survived")}
	t.Setenv("PREVIEW_HELPER", "1")
	t.Setenv("READY_FILE", files.ready)
	t.Setenv("RELEASE_FILE", files.release)
	t.Setenv("SURVIVED_FILE", files.survived)
	t.Cleanup(func() { _ = os.WriteFile(files.release, []byte("release"), 0o600) })
	exe, err := os.Executable()
	require.NoError(t, err)
	// A copy in a directory with a space checks the shell's quoting.
	copied := filepath.Join(dir, "preview helper.exe")
	//nolint:gosec // copies this test binary.
	data, err := os.ReadFile(exe)
	require.NoError(t, err)
	//nolint:gosec // writes under t.TempDir().
	require.NoError(t, os.WriteFile(copied, data, 0o600))
	return files, `"` + copied + `" -test.run=TestHelperProcess`
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		require.False(t, time.Now().After(deadline), "descendant process did not start")
		time.Sleep(10 * time.Millisecond)
	}
}

// assertNeverWritten releases the helper and checks it was killed before it
// could write survived.
func assertNeverWritten(t *testing.T, files helperFiles) {
	t.Helper()
	require.NoError(t, os.WriteFile(files.release, []byte("release"), 0o600))
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		_, err := os.Stat(files.survived)
		require.ErrorIs(t, err, os.ErrNotExist, "descendant process survived")
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRenderUsesPreviewCommandWithQuotedPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "has space & more")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "inside.txt"), nil, 0o600))
	out, err := Render(context.Background(), model.Session{Path: dir, PreviewCommand: "dir /b {}"}, "")
	require.NoError(t, err)
	assert.Equal(t, "inside.txt\r\n", out)
}

func TestRunShellCancellationKillsDescendantProcess(t *testing.T) {
	files, start := helper(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		_, _ = runShell(ctx, start)
		close(done)
	}()
	waitForFile(t, files.ready)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "runShell remained blocked after cancellation")
	}
	assertNeverWritten(t, files)
}

func TestRunShellCleansUpBackgroundDescendantAfterShellExit(t *testing.T) {
	files, start := helper(t)
	// start /b leaves the helper running after cmd.exe exits; the second
	// helper keeps cmd.exe alive until the first has started. The background
	// helper inherits cmd.exe's output pipes, so runShell stops waiting for
	// them after WaitDelay, then ends the job, as sh does on Unix.
	_, err := runShell(context.Background(), `start "" /b `+start+` & `+start+` -- wait`)
	require.ErrorIs(t, err, exec.ErrWaitDelay)
	_, err = os.Stat(files.ready)
	require.NoError(t, err, "descendant process did not start")
	assertNeverWritten(t, files)
}
