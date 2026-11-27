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
	return seshLua.Execute(a.Out, struct{ Bin, Key string }{Bin: bin, Key: *key})
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
