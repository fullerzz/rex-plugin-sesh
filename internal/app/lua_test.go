package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLuaPrintsTheActionForThisBinary(t *testing.T) {
	bin, err := os.Executable()
	require.NoError(t, err)
	if resolved, err := filepath.EvalSymlinks(bin); err == nil {
		bin = resolved
	}
	var out bytes.Buffer
	require.NoError(t, (&App{Out: &out, Err: io.Discard}).Run(context.Background(), []string{"lua"}))
	assert.Contains(t, out.String(), "local bin = "+luaString(bin)+"\n")
	assert.Contains(t, out.String(), `name = "sesh_picker"`)
	// The layer runs on the session's machine, which may lack rex-sesh.
	assert.Contains(t, out.String(), `local argv = { "/bin/sh", "-c", start, bin, "picker" }`)
	assert.Contains(t, out.String(), "local start = "+luaString(pickerStart)+"\n")
	// Only a key or the palette names the app that opened the picker.
	assert.Contains(t, out.String(), `if ctx.client_id and (ctx.origin == "key" or ctx.origin == "palette") then
      argv[#argv + 1] = "--opener"
      argv[#argv + 1] = ctx.client_id`)
	assert.NotContains(t, out.String(), `"--client"`)
	assert.NotContains(t, out.String(), "\nrex.bind(", "no key without --key")

	out.Reset()
	require.NoError(t, (&App{Out: &out, Err: io.Discard}).Run(context.Background(), []string{"lua", "--key", "ctrl+a>s"}))
	assert.Contains(t, out.String(), "\nrex.bind(\"ctrl+a>s\", \"sesh_picker\")\n")
}

func TestLuaStringEscapesForLua51(t *testing.T) {
	assert.Equal(t, `"a \"b\" \\c\n\001\127é"`, luaString("a \"b\" \\c\n\x01\x7fé"))
}

type countingReader struct {
	io.Reader

	reads int
}

func (r *countingReader) Read(p []byte) (int, error) {
	r.reads++
	return r.Reader.Read(p)
}

// A picker in a floating layer must keep its error on screen until Enter.
func TestPickerWaitsForEnterAfterAnError(t *testing.T) {
	in := &countingReader{Reader: strings.NewReader("\n")}
	var errb bytes.Buffer
	a := &App{In: in, Out: io.Discard, Err: &errb}
	err := a.Run(context.Background(), []string{"picker", "--wait-on-error", "--config", filepath.Join(t.TempDir(), "missing.toml")})
	require.Error(t, err)
	assert.Contains(t, errb.String(), "rex-sesh: "+err.Error()+"\n\nPress Enter to close.\n")
	assert.Positive(t, in.reads)
}

func TestPickerWaitsForEnterWhenRexIsDown(t *testing.T) {
	useFakeRex(t, `printf '%s\n' 'Error: no Rex server is running' >&2; exit 4`)
	in := &countingReader{Reader: strings.NewReader("\n")}
	var errb bytes.Buffer
	a := &App{In: in, Out: io.Discard, Err: &errb}
	err := a.Run(context.Background(), []string{"picker", "--json", "--wait-on-error", "--config", writeConfig(t, "version = 1\n")})
	require.EqualError(t, err, "list Rex sessions: rex do: no Rex server is running")
	assert.Contains(t, errb.String(), "Press Enter to close.")
	assert.Positive(t, in.reads)
}

func TestPickerDoesNotWaitWithoutAnError(t *testing.T) {
	in := &countingReader{Reader: strings.NewReader("\n")}
	var errb bytes.Buffer
	a := &App{In: in, Out: io.Discard, Err: &errb}
	require.NoError(t, a.Run(context.Background(), []string{"picker", "--json", "--wait-on-error", "--config", writeConfig(t, "version = 1\n")}))
	assert.NotContains(t, errb.String(), "Press Enter")
	assert.Zero(t, in.reads)
}

// runStart runs the layer's start script as Rex would, with bin as $0, and
// returns what it printed.
func runStart(t *testing.T, home, bin string) (string, error) {
	t.Helper()
	//nolint:gosec // the script is pickerStart, run on test fixtures.
	cmd := exec.Command("/bin/sh", "-c", pickerStart, bin, "picker", "--client", "client:mac")
	cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func writeEcho(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	//nolint:gosec // the fake must be executable.
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\necho \"$0 $*\"\n"), 0o700))
}

func TestPickerStartRunsThisMachinesBinaryWithItsArguments(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "rex-sesh")
	writeEcho(t, bin)
	out, err := runStart(t, t.TempDir(), bin)
	require.NoError(t, err)
	assert.Equal(t, bin+" picker --client client:mac\n", out)
}

// On another host, the Mac's path is missing and the client ID is the Mac
// server's, so the host's own copy runs without it.
func TestPickerStartFallsBackToTheHostsCopy(t *testing.T) {
	home := t.TempDir()
	hostBin := filepath.Join(home, ".local", "bin", "rex-sesh")
	writeEcho(t, hostBin)
	out, err := runStart(t, home, "/Users/me/rex-sesh/bin/rex-sesh")
	require.NoError(t, err)
	assert.Equal(t, hostBin+" picker\n", out)
}

func TestPickerStartSaysHowToInstallWhenMissing(t *testing.T) {
	out, err := runStart(t, t.TempDir(), "/Users/me/rex-sesh/bin/rex-sesh")
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 127, exitErr.ExitCode())
	assert.Contains(t, out, "Sesh cannot start: rex-sesh is not installed on this machine.\n")
	assert.Contains(t, out, "just install-host HOST")
}
