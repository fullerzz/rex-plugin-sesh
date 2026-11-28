package app

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/fullerzz/rex-plugin-sesh/internal/launcher"
	"github.com/fullerzz/rex-plugin-sesh/internal/rex"
)

//go:embed sesh.lua.tmpl
var seshLuaTemplate string

var seshLua = template.Must(template.New("sesh.lua").Funcs(template.FuncMap{"lua": luaString, "luaList": luaList}).Parse(seshLuaTemplate))

// pickerStart is the shell script the picker's layer runs on a Linux or
// macOS machine, as sh -c pickerStart BIN picker [--opener ID]. The layer
// runs on the machine that holds the session, which for a session on another
// host is not this one. There BIN is missing, so the script runs that host's
// own copy, which "just install-host" puts in ~/.local/bin, and leaves out
// --opener, because the app has another client ID on that host's server. A
// Windows host runs launcher.WindowsPicker instead.
const pickerStart = `if [ -x "$0" ]; then exec "$0" "$@"; fi
for c in "$HOME/.local/bin/rex-sesh" "$(command -v rex-sesh)"; do
  if [ -n "$c" ] && [ -x "$c" ]; then exec "$c" "$1"; fi
done
printf '%s\n' "Sesh cannot start: rex-sesh is not installed on this machine." \
  "On this machine, rebuild it with: just build" \
  "On another host, install it from your main machine with: just install-host SSH_DESTINATION" >&2
exit 127`

// lua prints the Rex config module that adds the picker action, pointing at
// this rex-sesh binary. --key also binds a key to it.
func (a *App) lua(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("lua", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	key := fs.String("key", "", "a Rex key to bind, such as ctrl+a>s")
	lastKey := fs.String("last-key", "", "a Rex key for the previous session, such as ctrl+a>shift+l")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("lua accepts only --key and --last-key")
	}
	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find the rex-sesh binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(bin); err == nil {
		bin = resolved
	}
	rexBin, lookupErr := exec.LookPath(rex.Bin())
	if lookupErr == nil {
		rexBin, err = filepath.Abs(rexBin)
		if err != nil {
			return fmt.Errorf("resolve Rex executable path: %w", err)
		}
	} else {
		rexBin = ""
	}
	return seshLua.Execute(a.Out, struct {
		Bin, RexBin, Key, LastKey, Start, OS string
		WindowsStart                         []string
	}{
		Bin: bin, RexBin: rexBin, Key: *key, LastKey: *lastKey, Start: pickerStart,
		OS: runtime.GOOS, WindowsStart: launcher.WindowsPickerArgv(),
	})
}

// luaString quotes s as a Lua 5.1 string literal. Lua has no \x or \u
// escapes, so control bytes use decimal \ddd and other bytes pass through.
func luaString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString(`\n`)
		case c < 0x20 || c == 0x7f:
			fmt.Fprintf(&b, `\%03d`, c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// luaList writes items as a Lua table of strings.
func luaList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = luaString(item)
	}
	return "{ " + strings.Join(quoted, ", ") + " }"
}
