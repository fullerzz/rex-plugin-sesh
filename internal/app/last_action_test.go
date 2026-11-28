package app

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fullerzz/rex-plugin-sesh/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLastCLIActionSelectsAppAttachedToInvokingSession(t *testing.T) {
	for _, host := range []string{"", "devbox"} {
		t.Run("host="+host, func(t *testing.T) {
			useHistoryDir(t)
			t.Setenv("REX_BLOCK", "")
			t.Setenv("REX_CLIENT", "")
			log := filepath.Join(t.TempDir(), "rex.log")
			t.Setenv("FAKE_REX_LOG", log)
			t.Setenv("FAKE_REX_SESSIONS", `{"sessions":[{"session_id":"a","label":"A"},{"session_id":"b","label":"B"},{"session_id":"terminal","label":"Terminal"}]}`)
			const script = `printf '%s\n' "$*" >> "$FAKE_REX_LOG"
case "$*" in
  *"-S devbox client ls --json -s terminal") clients='{"client_id":"client:r2","info":{"kind":"app","label":"Tablet"}}' ;;
  *"-S devbox client ls --json") clients='{"client_id":"client:r1","info":{"kind":"app","label":"Mac"}},{"client_id":"client:r2","info":{"kind":"app","label":"Tablet"}}' ;;
  *"client ls --json -s terminal") clients='{"client_id":"client:one","info":{"kind":"app","label":"Mac"}}' ;;
  *"client ls --json") clients='{"client_id":"client:one","info":{"kind":"app","label":"Mac"}},{"client_id":"client:two","info":{"kind":"app","label":"Tablet"}}' ;;
  *"hosts --json") echo '{"hosts":[]}'; exit ;;
  *"session.select"*) exit ;;
  *) echo "$FAKE_REX_SESSIONS"; exit ;;
esac
printf '{"clients":[%s]}' "$clients"`
			useFakeRex(t, script)
			args := []string{"last", "--context-session", "terminal"}
			wantApp := "client:one"
			if host != "" {
				args = append(args, "--context-host", host)
				wantApp = "client:two"
			}
			key := "id:" + wantApp
			recordSwitches(t, key, [2]state.Visit{localVisit("a"), localVisit("b")})
			require.NoError(t, (&App{}).Run(t.Context(), args))
			assert.Equal(t, []string{"--autostart=false -C " + wantApp + " do session.select session_id=a"}, selectCalls(t, log))
			assert.Equal(t, []state.Visit{localVisit("a"), localVisit("b")}, historyOf(t, key))
			if host != "" {
				// The invoking app disconnects from the config server. Its
				// one remaining client belongs to someone else.
				useFakeRex(t, strings.Replace(script, `,{"client_id":"client:two","info":{"kind":"app","label":"Tablet"}}`, "", 1))
				require.ErrorContains(t, (&App{}).Run(t.Context(), args), "none is this machine's app")
				assert.Len(t, selectCalls(t, log), 1)
			}
		})
	}
}

func TestLuaLastCLIForwardsSessionWithoutOpener(t *testing.T) {
	lua, err := exec.LookPath("lua")
	if err != nil {
		t.Skip("lua interpreter unavailable")
	}
	var module bytes.Buffer
	require.NoError(t, (&App{Out: &module}).Run(t.Context(), []string{"lua"}))
	script := `local actions = {}
rex = {
  action = function(action) actions[action.name] = action end,
  exec = function(argv) print(table.concat(argv, " ", 2)); return {ok=true} end,
}
` + module.String() + `
actions.sesh_last.run({origin="cli", session_id="s", client_id="client:cli"})
actions.sesh_last.run({origin="cli", session_id="s", server="devbox", client_id="client:cli"})
`
	//nolint:gosec // execute the generated module with a stub Rex API.
	cmd := exec.CommandContext(t.Context(), lua, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)
	assert.Equal(t, "last --context-session s\nlast --context-session s --context-host devbox\n", string(out))
}
