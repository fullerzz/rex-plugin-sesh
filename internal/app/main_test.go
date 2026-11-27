package app

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeRexLog records each call to the fake rex that TestMain installs.
var fakeRexLog string

// TestMain points the session cache at a temporary directory, rex at a fake
// with no sessions, and zoxide at an empty database, so no test reads the
// user's rex-sesh state or zoxide history or reaches the live Rex server. The fake uses only shell builtins: some tests empty
// PATH. It also clears the Rex terminal's own variables, which tests set.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "rex-sesh-app-test-")
	if err != nil {
		panic(err)
	}
	fakeRexLog = filepath.Join(dir, "rex.log")
	bin := filepath.Join(dir, "rex")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + fakeRexLog + "'\nprintf '%s' '{\"sessions\":{}}'\n"
	//nolint:gosec // the fake rex must be executable.
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		panic(err)
	}
	// _ZO_DATA_DIR gives zoxide, where installed, an empty database of its own.
	env := map[string]string{"REX_SESH_STATE_DIR": filepath.Join(dir, "state"), "REX_SESH_REX_BIN": bin, "_ZO_DATA_DIR": filepath.Join(dir, "zoxide")}
	for name, value := range env {
		if err := os.Setenv(name, value); err != nil {
			panic(err)
		}
	}
	for _, name := range []string{"REX_BLOCK", "REX_SESSION", "REX_CLIENT"} {
		_ = os.Unsetenv(name)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
