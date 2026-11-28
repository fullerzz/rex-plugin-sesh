package filelock

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelperProcess holds the lock named by FILELOCK_HELPER_PATH until its
// stdin closes. Other tests run it as a competing process.
func TestHelperProcess(t *testing.T) {
	path := os.Getenv("FILELOCK_HELPER_PATH")
	if path == "" {
		t.Skip("helper process only")
	}
	f, err := Open(path)
	if err != nil {
		os.Exit(2)
	}
	if err := TryLock(f); err != nil {
		os.Exit(3)
	}
	_, _ = os.Stdout.WriteString("locked\n")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	_ = Unlock(f)
	os.Exit(0)
}

func TestTryLockFailsWhileAnotherProcessHoldsTheLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "has space", "test.lock")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))

	//nolint:gosec // runs this test binary as the competing process.
	helper := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	helper.Env = append(os.Environ(), "FILELOCK_HELPER_PATH="+path)
	stdin, err := helper.StdinPipe()
	require.NoError(t, err)
	stdout, err := helper.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, helper.Start())
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "locked\n", line)

	f, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	require.ErrorIs(t, TryLock(f), ErrWouldBlock)

	require.NoError(t, stdin.Close())
	require.NoError(t, helper.Wait())
	require.NoError(t, TryLock(f))
	assert.NoError(t, Unlock(f))
}

// Each open file is its own owner, even within one process.
func TestTryLockFailsForASecondOpenOfTheSameFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")
	first, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = first.Close() }()
	second, err := OpenNoFollow(path)
	require.NoError(t, err)
	defer func() { _ = second.Close() }()

	require.NoError(t, TryLock(first))
	require.ErrorIs(t, TryLock(second), ErrWouldBlock)
	require.NoError(t, Unlock(first))
	require.NoError(t, TryLock(second))
	assert.NoError(t, Unlock(second))
}

func TestOpenNoFollowRejectsASymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	require.NoError(t, os.WriteFile(target, nil, 0o600))
	link := filepath.Join(dir, "test.lock")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
	f, err := OpenNoFollow(link)
	if err == nil {
		_ = f.Close()
	}
	require.Error(t, err)
	f, err = Open(link)
	require.NoError(t, err, "Open follows links")
	assert.NoError(t, f.Close())
}

func TestOpenNoFollowRejectsADirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")
	require.NoError(t, os.Mkdir(path, 0o700))
	f, err := OpenNoFollow(path)
	if err == nil {
		_ = f.Close()
	}
	assert.Error(t, err)
}
