package app

import (
	"bytes"
	"context"
	"io"
	"os"
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
	assert.Contains(t, out.String(), `Sesh cannot start: $0 is not on this machine.`)
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
