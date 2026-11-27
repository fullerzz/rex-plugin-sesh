package app

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed sesh.lua.tmpl
var seshLuaTemplate string

var seshLua = template.Must(template.New("sesh.lua").Funcs(template.FuncMap{"lua": luaString}).Parse(seshLuaTemplate))

// pickerStart is the shell script the picker's layer runs, as
// sh -c pickerStart BIN picker [--opener ID]. The layer runs on the machine
// that holds the session, which for a session on another host is not this
// one. There BIN is missing, so the script runs that host's own copy, which
// "just install-host" puts in ~/.local/bin, and leaves out --opener, because
// the app has another client ID on that host's server.
const pickerStart = `if [ -x "$0" ]; then exec "$0" "$@"; fi
for c in "$HOME/.local/bin/rex-sesh" "$(command -v rex-sesh)"; do
  if [ -n "$c" ] && [ -x "$c" ]; then exec "$c" picker; fi
done
printf '%s\n' "Sesh cannot start: rex-sesh is not installed on this machine." \
  "On this machine, rebuild it with: just build" \
  "On another host, install it from your main machine with: just install-host HOST" >&2
exit 127`

// lua prints the Rex config module that adds the picker action, pointing at
// this rex-sesh binary. --key also binds a key to it.
func (a *App) lua(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("lua", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	key := fs.String("key", "", "a Rex key to bind, such as ctrl+a>s")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("lua accepts only --key")
	}
	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find the rex-sesh binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(bin); err == nil {
		bin = resolved
	}
	return seshLua.Execute(a.Out, struct{ Bin, Key, Start string }{Bin: bin, Key: *key, Start: pickerStart})
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
