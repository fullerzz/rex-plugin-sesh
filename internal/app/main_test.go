package app

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain points the session cache at a temporary directory, so no test reads
// or writes the user's rex-sesh state.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "rex-sesh-app-test-")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("REX_SESH_STATE_DIR", filepath.Join(dir, "state")); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
