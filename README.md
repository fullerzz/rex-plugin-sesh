# rex-sesh

A [Sesh](https://github.com/joshmedeski/sesh)-style session picker for the Rex
terminal. Press a key, choose one of your Rex sessions or a
[zoxide](https://github.com/ajeetdsouza/zoxide) directory, and the Rex app
switches to it.

rex-sesh started as a fork of herdr-plugin-sesh, and work is in progress. See
[`PLAN.md`](PLAN.md) for what works today and what comes next. The pages in
`docs/` still describe the Herdr plugin and are being rewritten.

## Requirements

- The Rex app, with **Remote Control** turned on in its settings. rex-sesh
  switches sessions through the app's `session.select` action, and the app
  accepts actions from other programs only while Remote Control is on.
- [mise](https://mise.jdx.dev) to install the pinned Go toolchain and tools.
- Optional: `zoxide` to list your frequent directories, `eza` and `bat` for
  directory and README previews, and `fzf` for the `--fzf` picker.

## Build

```bash
mise install
just build
```

The binary is `bin/rex-sesh`. `just clean` deletes it, so rebuild before you
use the picker again.

## Set up the picker in Rex

1. Write the Rex config module. It points at the binary that prints it, and
   `--key` binds a key to open the picker:

   ```bash
   ./bin/rex-sesh lua --key 'ctrl+a>s' > ~/.config/rex/sesh.lua
   ```

2. Load it from `~/.config/rex/init.lua` by adding this line:

   ```lua
   require("sesh")
   ```

3. Check the config, then reload it:

   ```bash
   rex config check && rex config reload
   ```

4. Turn on Remote Control in the Rex app's settings.

Press the key, or run **Sesh: Pick Session** from the command palette. The
picker opens in a floating layer over the current tab.

The picker runs on the machine that holds the current session. It lists that
machine's Rex sessions, then the sessions on each host that `rex hosts` lists,
marked with the host's name, such as `[devbox] api`. A host that doesn't answer
within 3 seconds is left out.

Opened from a session on another host, the picker runs that host's own copy of
`rex-sesh`, from `~/.local/bin/rex-sesh` or `PATH`. It lists that host's
sessions, its hosts' sessions and its zoxide directories. Install or update the
copy from this machine over SSH, building for each host's OS and CPU:

```bash
just install-host devbox lab
```

Without a copy, the layer shows "Sesh cannot start". Close it with its ×
button.

To remove it, delete the `require("sesh")` line and `~/.config/rex/sesh.lua`,
then run `rex config reload`.

## Use the picker

| Key | Action |
| --- | --- |
| Type | Filter sessions by name or path. |
| `enter` | Switch the Rex app to the selected session and close the picker. On a directory, open a new Rex session there first. |
| `ctrl+j` / `ctrl+k`, arrows | Move the selection. |
| `ctrl+o` | Switch the preview between command output and the session's live screen. Directories keep their command preview. |
| `f2` | Open the settings editor. |
| `esc` | Close the picker. |

The picker lists your open Rex sessions first, then each host's sessions, then
your zoxide directories. A host's session switches through that host, and its
command preview shows the session's details, because its directory isn't on this
machine.
Directories are named after their last path element, with parent elements added
where two would share a name, such as `a/docs` and `b/docs`. A directory named
like an open session is left out. Choosing a directory
creates a Rex session named after it, starting in it, and adds it to zoxide.

`◆` marks the session the picker was opened from. Beside each session, a glyph
shows the most urgent status its programs report through Rex: a spinner while
working, `◉` blocked, `●` done, and `✓` idle. If the Rex app refuses a switch,
for example because Remote Control is off, the picker stays open and shows the
app's message.

If your Rex config binds a key the picker uses, the app takes the key first.
For example, binding `f2` in `init.lua` hides the settings editor; run
`rex-sesh config edit` instead.

## Configuration

Configuration is optional. rex-sesh reads its versioned TOML config from the
first location that exists:

1. `--config PATH`
2. `REX_SESH_CONFIG`
3. `${REX_SESH_CONFIG_DIR}/config.toml`
4. `${REX_SESH_CONFIG_DIR}/sesh.toml`, a legacy fallback
5. `~/.config/rex-sesh/config.toml`
6. `~/.config/rex-sesh/sesh.toml`, a legacy fallback
7. `~/.config/sesh/sesh.toml`, a legacy fallback

Legacy Sesh-compatible files still load, with a deprecation warning on stderr.
An explicit path may hold either schema; a top-level `version` key selects the
native format. `rex-sesh config migrate` converts a legacy file.

### Environment variables

| Variable | Effect |
| --- | --- |
| `REX_SESH_CONFIG` | Config file to load. |
| `REX_SESH_CONFIG_DIR` | Directory searched for `config.toml` and `sesh.toml`. |
| `REX_SESH_STATE_DIR` | Directory for the session cache. Defaults to `$XDG_STATE_HOME/rex-sesh` or `~/.local/state/rex-sesh`. |
| `REX_SESH_REX_BIN` | The `rex` binary to run. Defaults to the Rex server's own copy, then `rex` on `PATH`. |
| `REX_SESH_PICKER` | Set to `fzf` to use the fzf picker. |
| `REX_SESH_REDUCE_MOTION` | Turn off picker animations. |

## CLI

| Command | Purpose |
| --- | --- |
| `rex-sesh picker` | Open the picker over Rex sessions and zoxide directories, and switch to the chosen one. |
| `rex-sesh picker --client ID` | Switch a particular Rex app, by client ID or label. |
| `rex-sesh picker --wait-on-error` | After an error, wait for Enter before exiting. |
| `rex-sesh picker --fzf` | Use the fzf picker. |
| `rex-sesh picker --json` | Print the picker's sessions as JSON. |
| `rex-sesh list --json` | List Rex sessions on this machine and every host, config entries, and zoxide directories as JSON. |
| `rex-sesh preview TARGET` | Render the configured preview for a session. |
| `rex-sesh root` | Print the current Git repository root. |
| `rex-sesh lua [--key KEY]` | Print the Rex config module for the picker. |
| `rex-sesh config path` | Print the resolved config path. |
| `rex-sesh config init` | Create a starter config if none exists. |
| `rex-sesh config edit [--config PATH]` | Edit settings, with a review before saving. |
| `rex-sesh config validate [PATH]` | Validate a config and print its resolved path. |
| `rex-sesh config migrate` | Convert a legacy Sesh-style config to the native format. |

Without `--client`, the picker switches the app named by `REX_CLIENT`, or the
only connected Rex app. With several apps connected, pass `--client`. On a host,
the app has another client ID, so the picker switches the only app connected to
that host, or the one with the same device name as this machine's app.

## Development

Tool versions are pinned in [`mise.toml`](mise.toml), and common tasks live in
the [`justfile`](justfile).

```bash
mise install
just check
just build
```

Local builds embed a Git-derived version: the nearest `v*` tag, commits since
it, and the commit hash, or the commit hash alone before the first tag.
Uncommitted changes append `-dirty`.

## License

Licensed under the [MIT License](LICENSE).
