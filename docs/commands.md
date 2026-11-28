---
icon: lucide/terminal
---

# Commands

For daily use, open the picker from Rex with the `sesh_picker` action. See
[Rex setup](rex-app.md). The commands on this page are for scripts, checks
and the config.

## Run the binary

`just build` writes the binary to `bin/rex-sesh` in your checkout. Run it from
there, or link it onto your `PATH`:

```bash
ln -s "$PWD/bin/rex-sesh" ~/.local/bin/rex-sesh
```

rex-sesh talks to Rex through the `rex` command. It runs
`REX_SESH_REX_BIN` when that is set, then the Rex server's own `rex` from
`REX_BIN_DIR`, which Rex sets in its terminals, then `rex` on `PATH`. It never
starts a Rex server. Commands list the Rex sessions on the machine where they
run, and on the hosts that `rex hosts` lists there.

Put flags before other arguments, as in `rex-sesh preview --config PATH
TARGET`. Run `rex-sesh` with no arguments to list the commands. Add `-h` to
`picker`, `list`, `preview`, `lua`, `config edit` or `config migrate` to list
its flags.

## Command reference

| Command | Purpose |
| --- | --- |
| `--version` or `version` | Print the build version. |
| `picker [--config PATH] [--client ID] [--opener ID] [--fzf] [--json] [--wait-on-error]` | Open the picker and switch the Rex app to the chosen row, or print the rows as JSON. |
| `last [--client ID] [--opener ID]` | Switch directly to the app’s previous open session. |
| `list [--config PATH] [--json] [--blacklisted] [--hide-duplicates=false]` | List Rex sessions, hosts' sessions, config entries and zoxide directories. |
| `preview [--config PATH] TARGET` | Print the command preview for a session, config entry or directory. |
| `lua [--key KEY] [--last-key KEY]` | Print the Rex config module that registers the picker and last-session actions. |
| `root` | Print the root of the Git repository in the current directory. |
| `config path` | Print the config file in use, or where a new one would go. |
| `config init` | Create a starter config if none exists. |
| `config edit [--config PATH]` | Open the settings editor, with a review before saving. |
| `config validate [PATH]` | Validate a config and print its path. |
| `config migrate [PATH] [--force]` | Convert a legacy Sesh-style config to the native format. |
| `config migrate --config PATH [--force]` | The same, with the legacy file given as a flag. |

Errors go to stderr, and the command exits with status 1.

## `picker`

`rex-sesh picker` opens the picker. Its rows are this machine's open Rex
sessions, then each host's sessions, then zoxide directories, in
`source_order`. Config entries aren't listed. When the picker runs in a Rex
terminal, it marks the session that holds that terminal with `◆`.

Enter switches the Rex app to the selected session, and the picker exits once
the switch succeeds. On a directory, the picker first creates a Rex session
named after it, starting there. After a switch, the picker prints the chosen
row's name, and records the switch in the app's
[session history](rex-app.md#session-history).

| Flag | Effect |
| --- | --- |
| `--config PATH` | Load this config file instead of the usual [lookup](config.md). |
| `--client ID` | Switch this Rex app, given as a client ID or label. |
| `--opener ID` | The Rex app that opened the picker, given as a client ID. The [Lua action](rex-app.md) passes it for a key or the command palette. The picker switches that app unless `--client` names another, and records the session it is in as the one the app switched from. |
| `--fzf` | Use the fzf picker instead of the native picker. `REX_SESH_PICKER=fzf` does the same. |
| `--json` | Print the picker's rows as JSON and exit, without switching. |
| `--wait-on-error` | After an error, print it with "Press Enter to close." and wait for ++enter++ before exiting. |

If this machine's Rex server doesn't answer, the picker doesn't open. It fails
with `list Rex sessions:` and the reason, with or without `--json`. A host that
doesn't answer is left out. With `--json`, the picker prints a warning that
names the host.

### Which Rex app switches

For a session on this machine, or a directory, the picker uses the first of:

1. `--client`.
2. `--opener`.
3. `REX_CLIENT`.
4. The only Rex app connected to this machine's server.
5. With several apps connected, the only one attached to the session that holds
   the picker.

Otherwise the switch fails with "2 Rex apps are connected; choose one with
--client", with the number of apps. With no app connected, it fails with
"no Rex app is connected to switch".

For a host's session, the picker switches through that host's server, where
the Rex app has another client ID. `--client`, `--opener` and `REX_CLIENT` name
an app only on this machine's server. The picker uses the only app connected to the host.
With several, it uses the one whose device name matches the app it chose on
this machine, by the steps above.

The native picker stays open when a switch fails, and shows the error. The
fzf picker switches after fzf exits, so a failed switch ends the command with
the error.

## `last`

Run `rex-sesh last` with the picker closed. It looks up the most recent other
open session in the target app’s history and switches to it. If you picked
C from A, calling `last` twice returns you to A and then C, even if B sits
between them in Rex’s session list. The native picker shows the same target
in its footer.

A key or palette action supplies `--opener` to identify the current session.
Without that information the command uses the newest recorded visit as current.
You can target an app explicitly with `--client ID`. Closed sessions are
skipped. An unreachable previous host, empty history, or a refused switch
produces an error and preserves the visit order. A remote target requires
an identifiable connection for the same app on that host.

## `list`

`rex-sesh list` prints every source, in `source_order`: Rex sessions on this
machine and its hosts, `[[workspace]]` entries from your config, and zoxide
directories. Config entries that name another `machine` are left out.

```bash
rex-sesh list
rex-sesh list --json
```

The text output has one row per entry: the source, the name and the path,
separated by tabs. A host's session is named like `[devbox] api`. `--json`
prints the same entries as a JSON array; a Rex session has `session_id`, and a
host's session also has `machine`, the host's label.

| Flag | Effect |
| --- | --- |
| `--config PATH` | Load this config file. |
| `--json` | Print JSON instead of text. |
| `--blacklisted` | List only the entries that `list.blacklist` hides. |
| `--hide-duplicates=false` | Keep entries that share a name with an earlier entry. |

`list` doesn't fail when Rex is missing. If this machine's server doesn't
answer, it prints `warning: rex sessions unavailable:` and the reason, and
lists the other sources. A host that doesn't answer gives
`warning: rex host NAME unavailable:` and the reason.

When `list.cache` is on, `list` reuses its output for 5 seconds. It doesn't
use or write the cache with `--blacklisted` or `--hide-duplicates=false`, and
doesn't write it after a warning about Rex. The cache lives in
`REX_SESH_STATE_DIR`, or `rex-sesh` under `$XDG_STATE_HOME`
(default `~/.local/state`).

## `preview`

`rex-sesh preview TARGET` prints the command preview that the picker shows for
an entry. TARGET is one of:

- a session ID, on this machine or a host;
- the name or path of a session or config entry on this machine;
- a directory.

A host's session is found only by its session ID. For a host's session, the
preview runs nothing and prints the session's name, host, ID and path.
Otherwise it runs the entry's preview command, or `workspace_defaults.preview`,
with `{}` replaced by the path. The default is
`eza --icons=always --color=always -la {}`. The command has 5 seconds to
finish.

The live screen that the picker shows in pane preview isn't available from
this command.

## `lua`

`rex-sesh lua` prints the Rex config module, `sesh.lua`, that adds the
`sesh_picker` and `sesh_last` actions. It points at the binary that prints it.

```bash
./bin/rex-sesh lua --key 'ctrl+a>s' > ~/.config/rex/sesh.lua
```

`--key KEY` adds `rex.bind(KEY, "sesh_picker")`. Without it, the module ends
with a commented picker example. `--last-key KEY` independently binds `sesh_last`. See
[Rex setup](rex-app.md) for what the module does.

## `root`

`rex-sesh root` prints the top directory of the Git repository that holds the
current directory. It fails outside a Git repository.

## Configuration commands

The config commands follow the [config lookup order](config.md), not the
current directory.

- `config path` prints the config file that would load. If none exists, it
  prints where `config init` would create one: `config.toml` in
  `REX_SESH_CONFIG_DIR`, or in `~/.config/rex-sesh`. If `REX_SESH_CONFIG`
  names a missing file, it fails instead.
- `config init` creates a native starter config, only when no config exists
  anywhere in the lookup order. Otherwise it prints the existing file's path,
  legacy files included. If `REX_SESH_CONFIG` names a missing file, it creates
  the file there.
- `config edit [--config PATH]` opens the settings editor. With `--config`, it
  edits that file, and can create a missing one after you review and confirm.
  See [Keybindings](keybindings.md#settings-controls) for its keys.
- `config validate [PATH]` strictly validates the file in use, or PATH, and
  prints its path. It fails when there is no config file. A legacy file is
  valid, with a deprecation warning.
- `config migrate [PATH] [--force]` converts a legacy Sesh-style file to the
  native format, and keeps the source file. `--force` overwrites an existing
  native file. See [Configuration](config.md) for where it writes and when it
  refuses.

## Environment variables

| Variable | Effect |
| --- | --- |
| `REX_SESH_CONFIG` | Config file to load. |
| `REX_SESH_CONFIG_DIR` | Directory searched for `config.toml` and `sesh.toml`. |
| `REX_SESH_STATE_DIR` | Directory for the `list` cache and the session history. |
| `REX_SESH_REX_BIN` | The `rex` binary to run. |
| `REX_CLIENT` | The Rex app that `picker` switches when neither `--client` nor `--opener` is given. |
| `REX_SESH_PICKER` | Set to `fzf` to use the fzf picker. |
| `REX_SESH_REDUCE_MOTION` | Set to `1` or `true` to turn off the picker's animations. |
| `REX_SESH_SMEAR_PRESET` | The selection cursor's trail: `crisp` (default), `gooey` or `ghost`. |
