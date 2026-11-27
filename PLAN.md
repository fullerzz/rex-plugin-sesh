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
- `sources.RexSessions` (source name `rex`) lists each open session. Its label becomes the name, the focused terminal's directory becomes the path, and the session ID goes in `WorkspaceID` (now `SessionID`, see milestone 7).
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
- `WorkspaceID` still holds the Rex session ID. The rename to `SessionID` is a separate refactor (done in milestone 7).
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

## Milestone 6 status (2026-11-27)

Done on branch `pane-preview`, then committed to `main` under the standing approval.

- `ctrl+o` pane mode shows the session's live screen. The session script records a preview block for each session: the active tab's focused block, or its first tiled block when the focused one is the floating picker. `screen.lua` reads that block's size and VT text, and Go keeps the last `rows` lines. The picker refreshes it every second.
- Rows show agent status. The script reads `program_status` from every block in a session and keeps the most urgent state: blocked, then working, then done, then idle. Rex's `error` state counts as idle, because the picker has no glyph for it. The picker refreshes statuses every second with the same script.
- The current-session marker moved to the rail column as a sky `◆`, because the status column now shows agent glyphs and `done` is drawn as `●`. On the selected row, the selection rail replaces it.
- Checked live: `◆` with a working spinner on this session. Pane mode showed `alt-harness-rex-brainstorm`'s and `pipeline-state-diagram`'s screens.
- `just check` passes with 805 tests.

### Notes

- Each refresh runs the session script, which attaches to every session for a moment, once a second while the picker is open.
- The pane preview shows the visible screen only, not the scrollback.

## Milestone 7 status (2026-11-27)

Done on branch `session-id`, then committed to `main` under the standing approval.

- `model.Session.WorkspaceID` is now `SessionID`. In JSON output (`list --json`, `picker --json`) and the session cache, the key `workspace_id` is now `session_id`. A cache written before the rename expires in 5 seconds and is then rebuilt.
- `WorktreeRelation.ParentWorkspaceID` and `ParentWorkspaceName` are now `ParentSessionID` and `ParentSessionName`.
- In the picker, the names that hold Rex session IDs or lists of Rex sessions now say session: `RecentSessionIDs`, `LastSessionID`, `LastSessionUnknown`, `RexSessions`, `closingSessionID`, `sessionOrder`, `rexSessionIDs`, `sortRexSessions`, `agentSessionOrder` and `rexSessionsByID`.
- Renamed with `gopls rename`. No behaviour changes. `just check` passes with 805 tests.

### Left as they are

- The config keys `picker.workspace_sort`, `picker.show_last_workspace` and `picker.show_last_workspace_path`, the `[[workspace]]` config entries, and the Go names that mirror them (`WorkspaceSort`, `HideLastWorkspace`, `ShowLastWorkspace`). They keep herdr-sesh configs loading.
- The close machinery (`CloseWorkspace`, `workspaceCloseMsg`, `workspaceCloseContext`). Closing is refused until it is implemented.
- `state/history.go`, which nothing uses.

## Order of the remaining work (decided 2026-11-27)

1. Rename `WorkspaceID` to `SessionID` (milestone 7, done).
2. zoxide support (milestone 8).
3. Sessions on other hosts.
4. Rewrite the docs site and release tooling for Rex, once the port is in good shape.

## Milestone 8 status (2026-11-27)

Done on branch `zoxide`, then committed to `main` under the standing approval.

- The picker lists zoxide directories after the open Rex sessions, in `source_order` (default `rex, config, zoxide, dir`). Config entries stay out of the picker for now. `picker --json` and the reloads list the same rows.
- A zoxide directory is named after its last path element. Directories that share one get parent elements added until each name is unique, so `/src/a/docs` and `/src/b/docs` show as `a/docs` and `b/docs`. A directory named like an open Rex session is left out, so a directory you have opened shows once, as its session. Matching is by name, not path, because a session's path follows its terminal's directory.
- Fix (2026-11-27): the first version named every directory after its last element only, and the merge then kept one directory per name. It showed 241 of the 306 directories in this Mac's zoxide database. It now shows 304; the other 2 are named like open sessions.
- Enter on a directory finds the Rex app first, then runs `rex new NAME --cwd DIR --json` and selects the new session. Once the app shows it, `zoxide add DIR` raises the directory's rank. The footer reads `Opening NAME…`.
- Rex allows several sessions with one label, so a retry after a refused switch would create another session. The picker remembers the session it created for each directory and selects it again instead.
- Without an app to switch, or with a directory that no longer exists, nothing is created. A name that starts with `-` would read as a flag, so Rex names that session itself.
- In `ctrl+o` pane mode, a directory has no screen, so it keeps its directory preview, and the title says PREVIEW.
- App tests give zoxide an empty database through `_ZO_DATA_DIR`, so they never read or write the user's zoxide history.
- `just check` passes with 813 tests.
- Checked live: `picker --json` listed 4 Rex sessions and 241 zoxide directories. With a session named `borgboi` created by `rex new`, the picker showed one `borgboi` row, the Rex one. The probe session was then killed.

### Checked by the user (2026-11-27)

- Enter on a zoxide row in the real picker opened a new session in that directory, and the app showed it. The picker later listed it as the Rex session `borgboi`, with no `borgboi` directory row.

### Not checked yet

- Whether `rex new` alone moves the app to the new session. No Rex API reports what the app shows.

### Notes

- If a switch is refused and the picker is then closed, the created session stays open in Rex.
- If the remembered session is killed before a retry, the retry fails until the picker is reopened.
- zoxide rows are local to this Mac, and new sessions start on this Mac's server.

## Milestone 9 status (2026-11-27): sessions on other hosts

Done on branch `hosts`, then committed to `main` under the standing approval.

- The picker opened from a Mac session lists this Mac's sessions, then each host's sessions in `rex hosts` order, then zoxide directories. Host rows have `Machine` set to the host label and show as `[rex-buntu26] lunar-dune`. `list` includes them too.
- Hosts are listed in parallel, at most 3 seconds each. A host that fails is left out, `list` and `picker --json` print `rex host NAME unavailable: …`, and the TUI says nothing. Only a failure of this Mac's server stops the picker from opening.
- Enter on a host row runs `rex -S HOST -C APP do session.select session_id=ID`. The app has another client ID on each host, so `--client` and `REX_CLIENT` only work for this Mac's server. On the host, the picker uses the only connected app. With several, it uses the one whose `info.label` (the device name) matches the app it picked on this Mac.
- `ctrl+o` pane mode reads a host session's screen through the host. The command preview shows `session`, `host`, `id` and `path`, because the directory isn't on this Mac.
- The status refresh covers every host. While a host doesn't answer, its sessions keep their last status instead of losing it every second.
- `just check` passes with 825 tests.

### Facts found

- The Mac app is a client of every host's server, with a different client ID on each, and the same `info.label` (`Zach’s MacBook Pro`).
- A `session.select` with a made-up ID, sent through either this Mac's server or `rex-buntu26`'s, reached the app. The app answered `SessionNotFoundOnServerError`, so it looks a session up on the server the request came through. A host's session must be selected through that host.
- `rex -S HOST do -` runs its Lua on this Mac and sends the API calls to the host: `os.getenv("HOME")` returned the Mac's home. Scripts can also reach hosts with `rex.servers()` and `rex.server(LABEL)`, which could later list every host in one `rex` process.
- Listing takes about 70 ms per host over Tailscale. `picker --json` with 5 Mac sessions, 2 host sessions and 304 directories takes 0.25 s.
- `rex.ui.pick` is still missing on this Mac's server and on `rex-buntu26`.

### Checked live

- Opened unfocused, the picker showed 5 Mac sessions with `◆` on this one, then `[rex-buntu26] lunar-dune` and `[rex-buntu26] sunlit-delta`, then zoxide. On `lunar-dune`, pane mode showed its screen from `rex-buntu26`, and command mode showed its summary. Esc closed the layer.

### Not checked yet

- Enter on a host row. A switch moves the user's app, so the user checks it.

### Decision: the picker opened inside a host's session (2026-11-27)

Decide after the user tries the Mac picker with host sessions. Until then it shows "Sesh cannot start". A copy of `rex-sesh` on a host would list that host's server as its own. Whether it could reach this Mac's sessions is untested.
