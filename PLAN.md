# rex-plugin-sesh: plan from herdr-plugin-sesh

Date: 2026-11-27. Rex app 0.1.0-preview.36, Rex server 0.1.0-preview.19 (control API revision 14).

## Summary

Fork herdr-plugin-sesh and strip it. Keep the Bubble Tea/bloom picker, the session model, the theme, and the preview code. Replace the Herdr client with a Rex client that runs the `rex` CLI. Replace the plugin manifest with a Lua snippet for `init.lua`. A custom action opens the picker in a floating layer. The picker calls the app's `session.select` action and then exits, and Rex closes the layer.

## Decisions (2026-11-27)

- Reuse: fork and strip. Create a new repo, copy the code, and delete the Herdr parts. Extract a shared core later only if real duplication appears.
- Switching: require Remote Control in v1. On refusal, the picker stays open and shows the app's message. The README tells users to turn on Remote Control. The `rex.ui.pick` palette fallback waits for a server with control API revision 17.
- History: keep the Herdr commit history, and delete the Herdr release tags. Versions start at the first Rex tag.
- Milestone 1 landed as one commit, and `main` was moved to it.

## What Rex provides (verified on this machine)

| Need | Rex mechanism | Verified |
| --- | --- | --- |
| Session list with metadata | One `rex do - --json` Lua script: `session.list`, then `session.attach` + `session.view` per session, then the `pwd` block method on each focused block | Yes, about 20 ms for 5 sessions |
| Picker as an overlay | `rex.session.new_layer{ window_id, bounds, focus = true, layout = rex.layout.block{ flavor = "com.superlogical.terminal.shell", options = { command = argv } } }` in a `rex.action` | Yes (the user's `floating_lazygit` action uses it) |
| Layer cleanup | The layer closes when its program exits | Yes |
| Switch the app's session | Client action `session.select` (`session_id`, optional `window_id`): `rex -C CLIENT do session.select session_id=ID` | Exit 0 with Remote Control on. Not tested with it off |
| Preview | `rex capture -s SESSION -b BLOCK --trim` | Yes, across sessions |
| Create a session (later, for zoxide) | `rex new LABEL --cwd DIR --json`, then `session.select` | Documented, not tested |
| Agent status | Block method `program_status` (OSC 7501: working, blocked) | Documented, not tested |
| Key binding | `rex.bind("ctrl+a>s", "sesh_picker")` in `init.lua`, then `rex config reload` | Documented |

### Caveats

- `session.view` fails for a session the connection hasn't attached. Call `session.attach` first.
- `pwd` returns a `file://host/path` URL. Decode it as `floating_lazygit` does.
- `block.method` takes the short name: `method = "pwd"`, not `com.superlogical.terminal.pwd`.
- No `REX_CLIENT` is set in panes. The action must pass `ctx.client_id` to the picker in argv.
- `rex.ui.pick` (the native palette picker) is nil here. It needs control API revision 17, and this server reports 14.
- `init.lua` cannot hold event handlers. `rex.on` only works in `rex do` scripts. No server event reports that a client changed the session it shows.

## Main risk: Remote Control

The picker process runs after the launching action has returned. Its only way to switch sessions is a CLI-origin client action, and the app accepts those only when Remote Control is on in its settings. Remote Control is off by default.

Mitigations:

1. On refusal, keep the picker open and show the app's message, for example "Turn on Remote Control in Rex settings". Don't exit, because exiting closes the layer and the message with it.
2. Later, add a palette fallback: a pure-Lua action that calls `rex.exec{ "rex-sesh", "list", "--json" }`, then `rex.ui.pick`, then `rex.client.queue("session.select", { session_id = id })`. An action-origin queue probably needs no Remote Control. This is inferred from the docs and needs server API revision 17.

## Launch snippet (ship as `sesh.lua`; the user adds `require("sesh")` to `init.lua`)

```lua
rex.action{
  name = "sesh_picker",
  title = "Sesh: Pick Session",
  run = function(ctx)
    assert(ctx.block_id, "Sesh needs a focused terminal")
    local source = assert(rex.session.describe_block{ block_id = ctx.block_id })
    local window = assert(source.block.window_id, "The terminal must be in a tab")
    -- The picker finds its own session from REX_BLOCK.
    local argv = { "rex-sesh", "picker" }
    if ctx.client_id then
      argv[#argv + 1] = "--client"
      argv[#argv + 1] = ctx.client_id
    end
    return assert(rex.session.new_layer{
      window_id = window,
      bounds = { 0.1, 0.1, 0.8, 0.8 },
      focus = true,
      layout = rex.layout.block{
        flavor = "com.superlogical.terminal.shell",
        options = { command = argv },
      },
    })
  end,
}
rex.bind("ctrl+a>s", "sesh_picker")
```

If `--client` is missing, the picker lists the session's clients with `rex client ls -s SESSION --json`. It uses the only `app` client if there is exactly one, and otherwise reports an error.

## Package map: keep, replace, drop

### Keep (rename `herdr` references)

- `internal/picker`: `tea.go`, `layout.go`, `effects.go`, `theme.go`, `model.go`, `fzf.go`, `settings.go`. `tea.go` has about 200 herdr/workspace references, such as `Source == "herdr"`, worktree tree prefixes, agent status, and close-workspace.
- `internal/model`. Rename `WorkspaceID` to `SessionID` with gopls rename after the fork.
- `internal/uitheme`, `internal/preview` (bat directory preview), `internal/state` (JSON helpers), `internal/namer`.
- `internal/config` (Sesh TOML) and `internal/settings` (settings overlay). Strip the Herdr-native and migration parts later.
- `justfile`, `mise.toml`, `golangci` setup, and the docs toolchain.
- The bloom pin `bloomfx.dev/bloom v0.0.0-alpha.11`. Keep it until the port is green.

### Replace

| Herdr piece | Rex replacement |
| --- | --- |
| `internal/herdr` (`Client`, `Backend`, `CLIClient`, `AppClient`) | `internal/rex`: a narrow client over `rex` with the same `Runner` seam for fakes. Methods: `Sessions` (embedded Lua, `go:embed`), `Select(client, sessionID)`, `Capture(session, block)`, `Destroy(sessionID)`, `Create(label, cwd)` (later) |
| `sources/herdr_workspaces.go` | `sources/rex_sessions.go` |
| `app/picker_backend.go` | Rex implementation of `picker.Backend` (`CloseWorkspace` becomes `session.destroy`; `RefreshAgentStatuses` uses `program_status`) |
| `connect.Open` | Existing session: `session.select`. New session (zoxide or config, later): `rex new` and then `session.select` |
| `herdr-plugin.toml` (actions, overlay panes, events) | `sesh.lua` snippet with the action and binding, plus `rex-sesh lua` to print it |
| Pane preview (`herdr/pane_preview.go`) | `rex capture` of the session's focused block |

### Drop for v1

- `history_watcher_*`, `last_marker`, and the `last` command: no client-switch event exists. At most, record switches the picker makes.
- `startup` (tab and pane layouts), `clone`, remote machines, worktree relations, and Herdr config migration.
- The zoxide and directories sources (zoxide comes later, by request).

### Tests

- Regenerate the golden frames in `internal/picker/testdata/frames`.
- Port `fake_client.go` to a fake Rex runner.
- Add a regression test for the `file://` decoding and for client resolution.

## v1 milestones

1. Fork, rename the module and binary, delete the Drop list, and get `just test` green.
2. `internal/rex` with the embedded session script, `rex-sesh list --json`, and tests with a fake runner.
3. The picker shows Rex sessions only, with the current session marked from `--from-session`.
4. Selection calls `session.select`, with the in-picker error on refusal.
5. `sesh.lua` snippet, `rex-sesh lua`, and README setup steps (including Remote Control).
6. Preview from `rex capture` and agent status from `program_status`.

## Verified session script (prototype)

```lua
local list = assert(rex.call("session.list"))
local out = {}
for _, s in ipairs(list.sessions or {}) do
  local item = { session_id = s.session_id, label = s.label, windows = 0 }
  rex.call("session.attach", { session_id = s.session_id })
  local view = rex.call("session.view", { session_id = s.session_id })
  if view then
    item.windows = #(view.windows or {})
    for _, w in ipairs(view.windows or {}) do
      if w.window_id == view.active_window_id and w.focused_block_id then
        item.focused_block_id = w.focused_block_id
        item.pwd = rex.call("block.method", {
          session_id = s.session_id, block_id = w.focused_block_id,
          creator = "com.superlogical.terminal", method = "pwd",
        })
      end
    end
  end
  out[#out + 1] = item
end
return out
```

## Milestone 1 status (2026-11-27)

Done in commit `e5d8aeb`, which is now `main`. The `strip-herdr` branch and the 24 Herdr tags were deleted.

- The module is `github.com/fullerzz/rex-plugin-sesh` and the binary is `rex-sesh`. Commands: `list`, `preview`, `root`, `picker`, `config`, `--version`.
- Removed: `internal/herdr`, `connect`, `startup`, `clone`, the Herdr workspace source, remote previews, the history watcher, `last`, `window`, `plugin`, `app`, `herdr-plugin.toml`, `herdr-app/`, `demo/`, and the Herdr verify skill.
- `just check` passes with 755 tests (1,073 before the strip). `just build-docs` passes.

### Deviations from the plan

- The config and zoxide sources still feed `list` and `picker`. The plan dropped them for v1. Milestone 3 decides whether they stay beside Rex sessions.
- `root` stays, without `--connect`.
- New environment variables: `REX_SESH_CONFIG`, `REX_SESH_PICKER`, `REX_SESH_SMEAR_PRESET`, and `REX_SESH_REDUCE_MOTION` replace their `HERDR_SESH_*` names. `REX_SESH_CONFIG_DIR` and `REX_SESH_STATE_DIR` replace the Herdr plugin-host variables.
- The config directory is `~/.config/rex-sesh`. The cache directory is `$XDG_STATE_HOME/rex-sesh` (default `~/.local/state/rex-sesh`).
- The picker prints the chosen session's name. Switching comes in milestone 4.
- `docs/herdr-app.md` was restored, because the docs nav and pages link to it.
- `test-release-ref` was removed from `just check`, because it reads the removed `herdr-plugin.toml`.

### Still Herdr

- Picker text, icons and golden frames (milestone 3).
- `uitheme` reads `HERDR_CONFIG_PATH` to inherit Herdr's theme.
- `state/history.go`, which nothing uses now.
- The README, the docs site, `zensical.toml`, the release and changelog workflows, `cliff.toml`, and `.agents/setup` and `.agents/resume`.

## Milestone 2 status (2026-11-27)

Done on branch `rex-client`. The work is staged, not committed.

- `internal/rex` runs `rex --autostart=false do - --json` with the embedded `sessions.lua`. A missing server is an error, and no new server starts. The binary comes from `REX_SESH_REX_BIN`, then `$REX_BIN_DIR/rex`, then `rex` on PATH.
- `sources.RexSessions` (source name `rex`) lists each open session. Its label becomes the name, the focused terminal's directory becomes the path, and the session ID goes in `WorkspaceID`.
- `list`, `picker` and `preview` include Rex sessions first. The default `source_order` is `rex, config, zoxide, dir`. A `herdr` entry from a herdr-sesh config is treated as `rex`.
- When Rex is unavailable, `list` and `picker --json` print one warning and continue, and `list` skips writing its cache. The TUI and fzf pickers hold warnings until they exit. `preview` ignores the error.
- `Merge` keeps two open sessions that share a label. A configured entry with that label merges into the first.
- Checked against the real server: `rex-sesh list --json` shows the same sessions as `rex ls --json`, with decoded paths.
- `just check` passes with 771 tests. App tests use a fake `rex` installed by `TestMain`, and one test checks that the fake was the one called.

### Findings

- With `REX_SERVER` set to an unreachable socket, `rex` still connected to the local server: `rex whoami --json` showed the default endpoint. Remote hosts should pass an explicit `-S`.
- Known limit: `PathFromFileURL` drops the URL's host. A session on another host would show its remote directory as if it were local.
- Each listing attaches the script's connection to every session for a moment, to read its view.
- A missing `rex` binary gives the same single warning and exit 0, so CI without Rex still passes the smoke checks.
- An empty Lua table encodes as `{}`, so the client treats `{}` and `null` as an empty session list.
- The picker shows Rex rows with a plain `[rex]` badge. Milestone 3 adds the icon, colour and wording.

## Milestone 3 status (2026-11-27)

Done on branch `picker-rex`, then committed to `main` under the standing approval.

- The picker lists open Rex sessions only, on every path: the TUI, fzf, `--json`, and the reloads after a close or a settings save. `list` still merges every source.
- The session that holds the picker's own block (`REX_BLOCK`) is marked with a sky `●` in the status column. The Lua script walks floating layers to find it. A picker in a floating layer never supplies its session's directory, so the script falls back to the tab's tiled blocks. The launch snippet no longer needs `--from-session`.
- If Rex is unavailable, the picker fails before it opens: `list Rex sessions: …`. Reloads return the same error for the picker to show.
- Wording is now `rex / sesh`, `N sessions`, `SESSIONS`, `No matching sessions`, `Filter sessions`, and `LAST SESSION`. The footer's sort label reads `rex order` for the default mode and leaves out `ctrl+x`. Rex rows have their own icon and keep the sky colour. A session without a directory previews as `session: …`. Settings help says sessions, except where it names `[[workspace]]` config entries.
- `ctrl+x` is refused (the picker shakes) until closing is implemented. `ctrl+r` still cycles the sort modes, and all three keep `rex ls` order for now.
- The golden frames were regenerated. All 148 changed lines kept their width, and the changes are text only.
- Checked live: in an unfocused floating layer in this session's tab, the picker showed the 4 Rex sessions with `●` on this session and its directory intact. The command preview listed the first session's directory. After `ctrl+o`, the pane preview read "pane preview is not available yet".
- `just check` passes with 778 tests.

### Not done yet

- The focused-layer case (the picker as its tab's focused block) is untested until milestone 5 opens it from a key.
- Milestone 5: a picker that fails before opening (Rex unavailable) exits at once, and its floating layer closes with it. From a key binding, the error would flash and vanish, so the launch path must keep it readable.
- The fzf picker (`--fzf` or `REX_SESH_PICKER=fzf`) lists Rex sessions only but shows no `●` mark.
- Inert settings: `picker.workspace_sort` (`recent` and `agent` have no data), `picker.show_last_workspace` (the footer is hidden), and `ctrl+o` pane mode (shows "pane preview is not available yet" until milestone 6).
- `WorkspaceID` still holds the Rex session ID. The rename to `SessionID` is a separate refactor.
- `picker.herdr_theme_inherit` still reads Herdr's theme file.

## Milestone 4 status (2026-11-27)

Done on branch `select-session`, then committed to `main` under the standing approval.

- Enter switches the Rex app to the chosen session with the client action `session.select`. The native picker waits for the switch and quits only when it succeeds. If the app refuses, the picker stays open and shows `Could not switch: …` in its toast, and Enter can try again.
- While a switch is running, a second Enter and the settings key are refused, and a result for another session is ignored. The footer reads `Switching to <name>…`.
- The `●` row switches too: the session that holds the picker isn't necessarily the one the app shows.
- Which app: `--client`, then `REX_CLIENT`, then the only connected app. With several apps, the only one attached to the picker's session wins. Otherwise the error asks for `--client`.
- The fzf picker switches after fzf exits and returns any error.
- A switch may take 15 seconds, more than the 10 seconds `rex` waits for the app, so the picker never kills `rex` while the app may still switch.
- `just check` passes with 794 tests.
- Checked live: with no `--client`, the picker in an unfocused floating layer found the one app, switched it to `floral-quarry` (exit 0), and printed the name. I then switched the app back to `rex-sesh-plugin-design`. No Rex API reports which session an app is showing, so "back" assumes the app was showing that session.

### Not done yet

- A refusal with Remote Control off is untested, so the refusal text is unknown.
- Several connected apps are covered by tests only.

## Milestone 5 status (2026-11-27)

Done on branch `rex-action`, then committed to `main` under the standing approval.

- `rex-sesh lua [--key KEY]` prints the Rex config module, from the template `internal/app/sesh.lua.tmpl`. It points at the binary that printed it. Paths and keys are quoted for Lua 5.1, which has no `\x` or `\u` escapes.
- The `sesh_picker` action ("Sesh: Pick Session") opens `rex-sesh picker --wait-on-error` in a focused floating layer over the current tab. It passes `--client ctx.client_id` only for the `key` and `palette` origins. From the CLI, `ctx.client_id` is the `rex` command itself, so the picker finds the app instead. `focus=false` opens it without moving focus.
- `--wait-on-error` prints the error and waits for Enter. The action no longer passes it: Rex already keeps a failed command's layer open, with an `Exited` badge, so waiting meant dismissing twice. The flag stays for other callers.
- The README is rewritten for Rex: build, setup in Rex, keys, config lookup order, environment variables, and the CLI.
- Checked: `rex config check` accepts the generated module, with and without `--key`, and with a path containing a space, quotes and a backslash.

### Notes

- In this user's config, `f2` is bound to `client.tab.rename`, so the app takes it before the picker sees it. The picker's settings key is unreachable there; `rex-sesh config edit` still works.
- Pressing the key again while a picker is open probably opens a second picker.
- The binding points at `bin/rex-sesh`. `just clean` deletes it, so rebuild after cleaning.
- The fzf picker's failed switch waits through the same path, but no test covers it, because the tests have no fzf.

### Fix: the picker opened from a session on another host (2026-11-27)

The user's first key press was in `sunlit-delta`, a session on the remote host `rex-buntu26`. The action ran from the Mac's config, but Rex created the layer's block on `rex-buntu26`, and there the Mac path to `rex-sesh` doesn't exist (`zsh:1: no such file or directory`). Rex has no option to start a block on another host.

- The action now runs `/bin/sh -c` and checks that `rex-sesh` is executable before starting it. If it isn't, the layer says "Sesh cannot start: <path> is not on this machine. Sesh runs on the machine that holds the current session." This also covers a binary removed by `just clean`.
- Checked from a scratch copy, unfocused: a bogus path prints the message and exits 127, and the layer stays. The real path opens the picker, and Esc closes it.
- A command that exits non-zero keeps its layer open with an `Exited` badge, and one that exits 0 closes it. The earlier note that a failing picker's layer vanishes was wrong.
- Verified by the user (2026-11-27): `ctrl+a s` in a session on this Mac opened the picker as the focused layer, and Enter switched the Rex app to the chosen session.

### Decision: sessions on other hosts (2026-11-27)

Mac sessions only for now. The picker lists and switches this Mac's sessions. Opened from a session on another host, it shows "Sesh cannot start". Revisit later, starting from the facts below.


- This Mac can list another host's sessions: `rex -S rex-buntu26 ls --json` works.
- `session.select` takes only a session ID, so whether the Mac app can show another host's session through it is unproven. `client.host.switch` takes no arguments.
- Under every option, opening the picker from a remote session needs `rex-sesh` on that host. A picker there must find the app itself, because the Mac's client ID would not resolve there.
