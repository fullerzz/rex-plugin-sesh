package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fullerzz/rex-plugin-sesh/internal/launcher"
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
	assert.Contains(t, out.String(), `if opener and ctx.client_id and (ctx.origin == "key" or ctx.origin == "palette") then
    argv[#argv + 1] = "--opener"
    argv[#argv + 1] = ctx.client_id`)
	assert.NotContains(t, out.String(), `"--client"`)
	assert.NotContains(t, out.String(), "\nrex.bind(", "no key without --key")

	out.Reset()
	require.NoError(t, (&App{Out: &out, Err: io.Discard}).Run(context.Background(), []string{"lua", "--key", "ctrl+a>s", "--last-key", "ctrl+a>shift+l"}))
	assert.Contains(t, out.String(), "\nrex.bind(\"ctrl+a>s\", \"sesh_picker\")\n")
	assert.Contains(t, out.String(), `rex.bind("ctrl+a>shift+l", "sesh_last")`)
	assert.Contains(t, out.String(), `local result, err = rex.exec(argv, { env = env, stdin = snapshot })`)
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
func runStart(t *testing.T, home, bin, command string) (string, error) {
	t.Helper()
	//nolint:gosec // the script is pickerStart, run on test fixtures.
	cmd := exec.Command("/bin/sh", "-c", pickerStart, bin, command, "--client", "client:mac")
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
	out, err := runStart(t, t.TempDir(), bin, "picker")
	require.NoError(t, err)
	assert.Equal(t, bin+" picker --client client:mac\n", out)
}

// On another host, the Mac's path is missing and the client ID is the Mac
// server's, so the host's own copy runs without it.
func TestPickerStartFallsBackToTheHostsCopy(t *testing.T) {
	home := t.TempDir()
	hostBin := filepath.Join(home, ".local", "bin", "rex-sesh")
	writeEcho(t, hostBin)
	out, err := runStart(t, home, "/Users/me/rex-sesh/bin/rex-sesh", "picker")
	require.NoError(t, err)
	assert.Equal(t, hostBin+" picker\n", out)
	out, err = runStart(t, home, "/Users/me/rex-sesh/bin/rex-sesh", "last")
	require.NoError(t, err)
	assert.Equal(t, hostBin+" last\n", out)
}

func TestPickerStartSaysHowToInstallWhenMissing(t *testing.T) {
	out, err := runStart(t, t.TempDir(), "/Users/me/rex-sesh/bin/rex-sesh", "picker")
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 127, exitErr.ExitCode())
	assert.Contains(t, out, "Sesh cannot start: rex-sesh is not installed on this machine.\n")
	assert.Contains(t, out, "just install-host SSH_DESTINATION")
}

// Execute the generated action when a Lua interpreter is available. Only the
// config-server process runner is provided, so launching a remote layer fails.
func TestLuaLastUsesConfigServerFromRemoteSession(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("REX_SESH_STATE_DIR", filepath.Join(dir, "custom"))
	require.NoError(t, os.MkdirAll(stateDir(), 0o700))
	const history = `{"apps":{"id:client:config":{"revision":9007199254740993}}}`
	require.NoError(t, os.WriteFile(filepath.Join(stateDir(), "session-history.json"), []byte(history), 0o600))
	lua, err := exec.LookPath("lua")
	if err != nil {
		t.Skip("lua interpreter unavailable")
	}
	var module bytes.Buffer
	require.NoError(t, (&App{Out: &module, Err: io.Discard}).Run(t.Context(), []string{"lua"}))
	script := `local actions = {}
local argv
rex = {
  action = function(action) actions[action.name] = action end,
  exec = function(args, opts) argv = args; print(opts.stdin); return { ok = true } end,
}
` + module.String() + `
actions.sesh_last.run({origin="key", client_id="client:config", session_id="session:9", server="devbox"})
for i=2,#argv do print(argv[i]) end
`
	//nolint:gosec // execute only the generated module with a stub Rex API.
	cmd := exec.CommandContext(t.Context(), lua, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)
	assert.Equal(t, []string{history, "last", "--opener", "client:config", "--opener-history-stdin", "--opener-session", "session:9", "--opener-host", "devbox"}, strings.Split(strings.TrimSuffix(string(out), "\n"), "\n"))
}

// runPickerAction runs the generated module's sesh_picker action in a Lua
// interpreter with a stub Rex API, as a key press from client:mac in the
// session ctx.server names, and returns the layer's command and shell. A
// host's server.version reports platforms[host]; a host missing from it
// fails. edit changes the module before it runs.
func runPickerAction(t *testing.T, server string, platforms map[string]string, edit func(string) string) (argv []string, shell string) {
	t.Helper()
	lua, err := exec.LookPath("lua")
	if err != nil {
		t.Skip("lua interpreter unavailable")
	}
	var module bytes.Buffer
	require.NoError(t, (&App{Out: &module, Err: io.Discard}).Run(t.Context(), []string{"lua"}))
	src := module.String()
	if edit != nil {
		src = edit(src)
	}
	var hosts strings.Builder
	for host, platform := range platforms {
		fmt.Fprintf(&hosts, "  [%s] = %s,\n", luaString(host), luaString(platform))
	}
	server = luaString(server)
	if server == `""` {
		server = "nil"
	}
	script := `local actions = {}
local platforms = {
` + hosts.String() + `}
local layer
rex = {
  action = function(action) actions[action.name] = action end,
  bind = function() end,
  server = function(host)
    return { call = function(method)
      assert(method == "server.version")
      return { platform = assert(platforms[host], "unreachable") }
    end }
  end,
  session = {
    describe_block = function() return { block = { window_id = "window:1" } } end,
    new_layer = function(l) layer = l; return true end,
  },
  layout = { block = function(b) return b end },
}
` + src + `
actions.sesh_picker.run({block_id="block:1", origin="key", client_id="client:mac", server=` + server + `}, {})
io.write(layer.layout.options.shell or "", "\0")
for _, a in ipairs(layer.layout.options.command) do io.write(a, "\0") end
`
	//nolint:gosec // execute only the generated module with a stub Rex API.
	cmd := exec.CommandContext(t.Context(), lua, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)
	fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	return fields[1:], fields[0]
}

func TestLuaPickerUsesShOnLinuxAndMacHosts(t *testing.T) {
	bin, err := os.Executable()
	require.NoError(t, err)
	if resolved, err := filepath.EvalSymlinks(bin); err == nil {
		bin = resolved
	}
	sh := []string{"/bin/sh", "-c", pickerStart, bin, "picker", "--opener", "client:mac"}
	for _, server := range []string{"", "devbox", "offline"} {
		argv, shell := runPickerAction(t, server, map[string]string{"devbox": "linux_amd64"}, func(src string) string {
			return strings.Replace(src, `local config_os = "`+runtime.GOOS+`"`, `local config_os = "linux"`, 1)
		})
		assert.Equal(t, sh, argv, "server %q", server)
		assert.Empty(t, shell, "server %q", server)
	}
}

// A Windows host has no /bin/sh. Its own copy runs without this machine's
// client ID.
func TestLuaPickerRunsAWindowsHostsOwnCopy(t *testing.T) {
	argv, shell := runPickerAction(t, "winbox", map[string]string{"winbox": "windows_amd64"}, nil)
	assert.Equal(t, launcher.WindowsPickerArgv(), argv)
	assert.Equal(t, "none", shell)
}

// On Windows, the module runs its own binary directly, with the opener.
func TestLuaPickerRunsThisWindowsMachinesBinary(t *testing.T) {
	bin := `C:\Users\Jo Doe\AppData\Local\Programs\rex-sesh\rex-sesh.exe`
	argv, shell := runPickerAction(t, "", nil, func(src string) string {
		src = strings.Replace(src, `local config_os = "`+runtime.GOOS+`"`, `local config_os = "windows"`, 1)
		i := strings.Index(src, "local bin = ")
		j := i + strings.Index(src[i:], "\n")
		return src[:i] + "local bin = " + luaString(bin) + src[j:]
	})
	assert.Equal(t, []string{bin, "picker", "--opener", "client:mac"}, argv)
	assert.Equal(t, "none", shell)
}

// On Windows, Go's os.UserHomeDir reads USERPROFILE even when HOME is set,
// so the module must look for history there too.
func TestLuaLastReadsWindowsHistoryUnderUserProfile(t *testing.T) {
	lua, err := exec.LookPath("lua")
	if err != nil {
		t.Skip("lua interpreter unavailable")
	}
	home, profile := t.TempDir(), t.TempDir()
	t.Setenv("REX_SESH_STATE_DIR", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", profile)
	write := func(root, history string) {
		dir := filepath.Join(root, ".local", "state", "rex-sesh")
		require.NoError(t, os.MkdirAll(dir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "session-history.json"), []byte(history), 0o600))
	}
	write(home, `{"from":"HOME"}`)
	write(profile, `{"from":"USERPROFILE"}`)
	var module bytes.Buffer
	require.NoError(t, (&App{Out: &module, Err: io.Discard}).Run(t.Context(), []string{"lua"}))
	src := strings.Replace(module.String(), `local config_os = "`+runtime.GOOS+`"`, `local config_os = "windows"`, 1)
	script := `local actions = {}
rex = {
  action = function(action) actions[action.name] = action end,
  bind = function() end,
  exec = function(args, opts) io.write(opts.stdin); return { ok = true } end,
}
` + src + `
actions.sesh_last.run({origin="key", client_id="client:config", session_id="session:9"})
`
	//nolint:gosec // execute only the generated module with a stub Rex API.
	cmd := exec.CommandContext(t.Context(), lua, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)
	assert.JSONEq(t, `{"from":"USERPROFILE"}`, string(out))
}

func TestLuaEmbedsAbsoluteRexPath(t *testing.T) {
	dir := t.TempDir()
	writeEcho(t, filepath.Join(dir, "rex"))
	t.Chdir(dir)
	t.Setenv("REX_SESH_REX_BIN", "./rex")
	var out bytes.Buffer
	require.NoError(t, (&App{Out: &out, Err: io.Discard}).Run(t.Context(), []string{"lua"}))
	cwd, err := os.Getwd()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "env.REX_SESH_REX_BIN = "+luaString(filepath.Join(cwd, "rex")))
}
