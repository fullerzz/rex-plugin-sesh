//go:build windows

package launcher

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With LAUNCHER_HELPER=1, this test binary stands in for rex-sesh.exe and
// prints its arguments.
func TestMain(m *testing.M) {
	if os.Getenv("LAUNCHER_HELPER") == "1" {
		fmt.Println(strings.Join(os.Args[1:], " "))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runPicker runs WindowsPickerArgv as Rex would with shell "none", with
// localAppData as %LOCALAPPDATA%.
func runPicker(t *testing.T, localAppData string) (stdout, stderr string, err error) {
	t.Helper()
	argv := WindowsPickerArgv()
	//nolint:gosec // runs the launcher script under test.
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), "LOCALAPPDATA="+localAppData, "LAUNCHER_HELPER=1")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err = cmd.Run()
	return out.String(), errb.String(), err
}

func TestWindowsPickerRunsTheHostsCopy(t *testing.T) {
	localAppData := filepath.Join(t.TempDir(), "App Data")
	bin := filepath.Join(localAppData, WindowsInstallPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(bin), 0o700))
	exe, err := os.Executable()
	require.NoError(t, err)
	//nolint:gosec // copies this test binary.
	data, err := os.ReadFile(exe)
	require.NoError(t, err)
	//nolint:gosec // writes under t.TempDir().
	require.NoError(t, os.WriteFile(bin, data, 0o600))

	stdout, stderr, err := runPicker(t, localAppData)
	require.NoError(t, err, stderr)
	assert.Equal(t, "picker", strings.TrimSpace(stdout))
}

func TestWindowsPickerSaysHowToInstallWhenMissing(t *testing.T) {
	_, stderr, err := runPicker(t, filepath.Join(t.TempDir(), "App Data"))
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 127, exitErr.ExitCode())
	assert.Contains(t, stderr, "Sesh cannot start: rex-sesh is not installed on this machine.")
	assert.Contains(t, stderr, `%LOCALAPPDATA%\Programs\rex-sesh\rex-sesh.exe`)
}
