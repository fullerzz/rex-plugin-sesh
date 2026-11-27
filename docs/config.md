---
icon: lucide/settings
---

# Configuration

Configuration is optional. Without a file, rex-sesh uses its built-in defaults.
The commands on this page run `rex-sesh`: use `./bin/rex-sesh` from your
checkout, or link it onto your `PATH` as [Commands](commands.md) shows.

The config file is TOML. The native format starts with `version = 1` and
rejects unknown keys. Older Sesh-style files still load, with a deprecation
warning; see [Legacy migration](#legacy-migration).

| To configure… | Start here |
| --- | --- |
| Picker appearance, search, sorting, and previews | [Picker](config/picker.md) |
| Settings from inside the picker | [Settings editor](#settings-editor) |
| Which rows the picker lists, and in what order | [`[list]`](#list) |
| Preview commands for your directories | [`[[rule]]`](#rule) and [`[workspace_defaults]`](#workspace_defaults) |
| Startup commands, tabs, and pane layouts | [Layouts](config/layouts.md). They are accepted but have no effect in rex-sesh. |

## Create your configuration

Create a starter file:

```bash
rex-sesh config init
```

If no config exists anywhere in the [lookup order](#configuration-file-lookup),
this writes `~/.config/rex-sesh/config.toml` and prints its path. With
`REX_SESH_CONFIG` set, it writes that file instead. With `REX_SESH_CONFIG_DIR`
set, it writes `config.toml` in that directory. If a config already exists,
`config init` prints its path and changes nothing.

The starter file looks like this:

```toml
version = 1

[workspace_defaults]
preview = "eza --icons=always --color=always -la {}"

# [[workspace]]
# name = "Example"
# path = "~/projects/example"
```

Add settings to it, for example:

```toml
version = 1

[picker]
show_icons = true
preview_mode = "pane"

[list]
blacklist = ["^scratch"]
```

Check the file, then reopen the picker:

```bash
rex-sesh config validate
```

The picker reads its config each time it opens. Settings saved in the
[settings editor](#settings-editor) apply at once.

The picker that Rex opens is started without `--config`, so it finds its config
through the lookup order. Opened in a session on another host, the picker runs
on that host and reads that host's config.

## Configuration file lookup

rex-sesh reads the first of these that exists:

1. `--config PATH`
2. `REX_SESH_CONFIG`
3. `${REX_SESH_CONFIG_DIR}/config.toml`
4. `${REX_SESH_CONFIG_DIR}/sesh.toml`, a legacy fallback
5. `~/.config/rex-sesh/config.toml`
6. `~/.config/rex-sesh/sesh.toml`, a legacy fallback
7. `~/.config/sesh/sesh.toml`, a legacy fallback

A path from `--config` or `REX_SESH_CONFIG` must exist. Only `config init`
(for `REX_SESH_CONFIG`) and `config edit` create a missing one; other commands
fail. That path may hold either format: a top-level `version` key selects the native
format, and a file without one is read as legacy. In the config directories, a
file named `config.toml` is always read as native, and one named `sesh.toml` is
always read as legacy.

`list`, `picker`, `preview`, `config edit` and `config migrate` accept
`--config`. `config validate` takes the path as an argument instead.

To see which file is in use:

```bash
rex-sesh config path
```

When no file exists, `config path` prints where `config init` would create
one. If `REX_SESH_CONFIG` names a missing file, it fails instead. See
[Commands](commands.md) for every command and flag.

## Settings editor

Press `f2` in the picker to edit settings. If `keys.cycle_preview_mode` is
`"f2"`, the settings key is `ctrl+,` instead. To edit settings without the
picker, run:

```bash
rex-sesh config edit
```

Add `--config PATH` to edit a particular file.

!!! note "When Rex takes `f2`"

    The Rex app handles keys that your Rex config binds before the picker sees
    them. If your `init.lua` binds `f2`, the picker never receives it. Run
    `rex-sesh config edit` instead. `rex keymap` shows what each key does.

The editor covers the `[picker]`, `[list]`, `[naming]`, `[keys]` and
`[workspace_defaults]` settings. Edit `[[workspace]]`, `[[rule]]`, `[[tab]]`
and `[history]` in the file.

Press `ctrl+s` to review your changes, then `y` to save. `ctrl+r` reverts the
selected setting. `esc` leaves the editor, and asks first if you have unsaved
changes. See [Keybindings](keybindings.md) for the full list of controls.

After a save from the picker, the picker reloads its settings and its rows,
and keeps your filter and selection where it can.

??? info "How the editor writes the file"

    The editor writes to the file the lookup order finds. If no file exists,
    it creates one where `config init` would, once you review and confirm.

    Only the settings you changed are rewritten; the rest of the file stays as
    it is. The new file replaces the old one in a single step and keeps its
    permissions. A new file gets `0600`. A `config.toml.settings.lock` file
    beside the config stops two editors from saving at once.

    If the file changed on disk after the editor opened it, the save fails
    and your draft stays. Press `r` to reload the file, then `y` to discard
    the draft.

    Saving also deletes the [`list` cache](#cache).

Opening a legacy file in the editor offers to convert it first. Press `y` to
create a native `config.toml`, as [`config migrate`](#legacy-migration) does.
The legacy file stays.

## Native and legacy formats

| | Native | Legacy (Sesh-style) |
| --- | --- | --- |
| Marker | `version = 1` at the top | No `version` key |
| File name in the config directories | `config.toml` | `sesh.toml` |
| Unknown keys | Rejected | Ignored, unless the file sets `strict_mode = true` |
| `import` of other files | Not supported | Supported |
| Loads with | No warning | A deprecation warning on stderr |

`rex-sesh config validate [PATH]` checks a file and prints its path. It reads a
legacy file strictly, so it rejects unknown keys that a normal load ignores,
such as Sesh's tmux settings.

## Settings

The picker's own settings, `[picker]` and `[keys]`, are on the
[Picker](config/picker.md) page.

### `[list]`

| Field | Default | Effect |
| --- | --- | --- |
| `source_order` | `["rex", "config", "zoxide", "dir"]` | The order of the row groups. See [Sources](#sources). |
| `blacklist` | `[]` | Regular expressions. A session or directory whose name matches one is hidden. See [Blacklist](#blacklist). |
| `cache` | `false` | Caches `rex-sesh list` output for five seconds. See [Cache](#cache). |

#### Sources

| Name | Rows |
| --- | --- |
| `rex` | Open Rex sessions: this machine's first, then each host's. |
| `zoxide` | Directories from your zoxide database. |
| `config` | `[[workspace]]` entries. Only `rex-sesh list` shows them. |
| `dir` | A directory given to `rex-sesh preview`. It never appears as a row. |
| `herdr` | Read as `rex`, so a config written for herdr-sesh keeps working. |

The picker lists only the `rex` and `zoxide` groups. `rex-sesh list` also
lists `config` entries. A source you leave out of `source_order` is added after
the ones you list. Unknown and repeated names are rejected.

!!! note "Sessions and directories with the same name"

    rex-sesh shows one row per name. A directory named like an open session on
    this machine is replaced by the session, whichever comes first in
    `source_order`. The row stays in the directory's place when `zoxide`
    comes first.

A legacy `sort_order` is the same setting. A legacy file that leaves out `rex`
(or `herdr`) lists Rex sessions after the sources it names.

#### Blacklist

Each pattern is a Go regular expression matched against the row's name: a
session's Rex name, without the `[host]` prefix, or a directory's picker name,
such as `a/docs`. The patterns apply to the picker and to `rex-sesh list`.
`rex-sesh list --blacklisted` shows only the hidden rows.

A legacy file skips an invalid pattern without an error. The native format
rejects it.

#### Cache

With `cache = true`, `rex-sesh list` reuses its last output for five seconds.
The cache is `sessions.json` in `REX_SESH_STATE_DIR`, or in
`$XDG_STATE_HOME/rex-sesh` (by default `~/.local/state/rex-sesh`). It belongs
to one config file. `list` skips it with `--blacklisted` or
`--hide-duplicates=false`, and doesn't write it while Rex or a host can't be
reached. The picker never uses the cache.

### `[naming]`

| Field | Default | Effect |
| --- | --- | --- |
| `path_components` | `1` | Must be at least `1`. Accepted, but has no visible effect in rex-sesh. |

Directory names in the picker come from zoxide paths: the last path element,
with parent elements added where two directories would share a name, such as
`a/docs` and `b/docs`. Rex sessions keep their Rex names.

### `[history]`

| Field | Default | Effect |
| --- | --- | --- |
| `last_workspace_label` | `"last"` | Accepted, but has no effect. rex-sesh records no session history. |

The value must be at most 80 characters, must not contain control characters,
and must not start or end with whitespace.

### `[workspace_defaults]`

| Field | Default | Effect |
| --- | --- | --- |
| `preview` | `eza --icons=always --color=always -la {}` | The preview command for a session or directory that has no other. An empty value uses the default. |
| `startup` | None | Accepted, but has no effect. rex-sesh runs no startup commands. |

`{}` in a preview command becomes the row's path, quoted for the shell. The
command runs with `sh -lc` and stops after five seconds. For a Rex session, the
path is the directory its focused terminal last reported.

### `[[workspace]]`

A `[[workspace]]` entry names a directory. `rex-sesh list` shows it with the
source `config`, and `rex-sesh preview NAME` previews it with its own preview
command. The picker doesn't list these entries. In `list`, an entry named like
an open Rex session merges into that session.

| Field | Effect |
| --- | --- |
| `name` | Required. Must be unique. |
| `path` | Required. `~/` expands to your home directory. |
| `preview` | Preview command for this entry. Empty uses the matching rule, then `workspace_defaults.preview`. |
| `machine` | A herdr-sesh setting for another machine. `list` leaves out an entry that sets it, and `preview NAME` shows only the entry's details. |
| `startup`, `disable_startup`, `tabs` | Accepted, but have no effect. See [Layouts](config/layouts.md). |

### `[[rule]]`

A rule gives a preview command to every session and directory on this machine
whose path matches its glob. It applies in the picker too. The first matching
rule wins.

| Field | Effect |
| --- | --- |
| `path_glob` | Required. `*`, `?` and character classes match as in Go's `filepath.Match`. A trailing `/**` matches the directory and everything under it. `~/` expands to your home directory. |
| `preview` | Preview command for a matching path. |
| `startup`, `disable_startup`, `tabs` | Accepted, but have no effect. See [Layouts](config/layouts.md). |

For example, show recent commits for anything under `~/src`:

```toml
[[rule]]
path_glob = "~/src/**"
preview = "git -C {} log --oneline --color=always -n 30"
```

A preview command is chosen in this order: the `[[workspace]]` entry's own,
then the first matching rule's, then `workspace_defaults.preview`, then the
built-in default. A session on another host shows its details instead of a
preview command.

## Environment variables

| Variable | Effect |
| --- | --- |
| `REX_SESH_CONFIG` | Config file to load. It must exist. |
| `REX_SESH_CONFIG_DIR` | Directory searched for `config.toml` and `sesh.toml`, before `~/.config/rex-sesh`. |
| `REX_SESH_STATE_DIR` | Directory for the `list` cache. Defaults to `$XDG_STATE_HOME/rex-sesh` or `~/.local/state/rex-sesh`. |
| `REX_SESH_REX_BIN` | The `rex` binary to run. Defaults to `rex` in `REX_BIN_DIR`, which Rex sets in its terminals, then `rex` on `PATH`. |
| `REX_SESH_PICKER` | Set to `fzf` to use the fzf picker. |
| `REX_SESH_REDUCE_MOTION` | Set to `1` or `true` to turn off picker animations. |
| `REX_SESH_SMEAR_PRESET` | Cursor animation: `crisp` (default), `gooey`, or `ghost`. See [Picker](config/picker.md#cursor-and-status-indicators). |
| `REX_CLIENT` | The Rex app the picker switches when `--client` isn't given. |
| `HERDR_CONFIG_PATH` | The Herdr config file that supplies picker colors. See [Picker colors](config/picker.md#picker-colors). |

## Legacy migration

Convert a legacy file to the native format:

```bash
rex-sesh config migrate
```

This converts the file the lookup order finds. Pass a path, or `--config PATH`,
to convert another one. The command prints the new file's path, and a warning
that says which file now loads.

- The new `config.toml` goes in the legacy file's directory. A file in
  `~/.config/sesh` converts to `~/.config/rex-sesh/config.toml`, or to
  `config.toml` in `REX_SESH_CONFIG_DIR`.
- The legacy file stays. Delete it once the new file works. If
  `REX_SESH_CONFIG` or `--config` names the old file, point it at the new one
  first.
- Imported files are merged into the new file.
- Keys rex-sesh doesn't know, such as Sesh's tmux settings, are left out.
- An existing `config.toml` is never replaced unless you pass `--force`, and
  `--force` replaces only a valid native file. A legacy file that is itself
  named `config.toml` must be renamed before you convert it.
- The new file must pass native validation, or nothing is written. For
  example, a Sesh `sort_order` that names `tmux` fails; replace `tmux` with
  `rex` first.

!!! warning "A stray `version` key in a legacy file"

    With `--config` or `REX_SESH_CONFIG`, a file that has a top-level
    `version` key is read as native, so its legacy keys fail. Remove the key,
    or convert the file. `config migrate` refuses a file with a `version` key.

??? info "Conversion details"

    A legacy file that never sets `tui.show_icons` gets `show_icons = true`.
    The old default preview `eza --icons=always -la {}` becomes
    `eza --icons=always --color=always -la {}`. An explicit icon setting and a
    custom preview command are kept. Comments and key order are not kept.

??? info "Legacy to native key reference"

    | Legacy key | Native key |
    | --- | --- |
    | `cache` | `list.cache` |
    | `strict_mode` | Removed; the native format is always strict |
    | `import` | Not supported; `config migrate` merges imported files |
    | `blacklist` | `list.blacklist` |
    | `sort_order` | `list.source_order` |
    | `dir_length` | `naming.path_components` |
    | `separator_aware` | `picker.separator_aware` |
    | `tui.show_icons` | `picker.show_icons` |
    | `tui.herdr_theme_inherit` | `picker.herdr_theme_inherit` |
    | `tui.replace_worktree_icon` | `picker.replace_worktree_icon` |
    | `tui.show_last_workspace` | `picker.show_last_workspace` |
    | `tui.show_last_workspace_path` | `picker.show_last_workspace_path` |
    | `tui.prompt` | `picker.prompt` |
    | `tui.placeholder` | `picker.placeholder` |
    | `tui.default_sort` | `picker.workspace_sort` (`workspace`, `recent`, or `agent`) |
    | `default_session.startup_command` | `workspace_defaults.startup` |
    | `default_session.preview_command` | `workspace_defaults.preview` |
    | `session[]` | `workspace[]` |
    | `session[].startup_command` | `workspace[].startup` |
    | `session[].preview_command` | `workspace[].preview` |
    | `session[].disable_startup_command` | `workspace[].disable_startup` |
    | `session[].windows` | `workspace[].tabs` |
    | `window[]` | `tab[]` |
    | `window[].startup_script` | `tab[].startup` |
    | `window[].path` | `tab[].path` |
    | `wildcard[]` | `rule[]` |
    | `wildcard[].pattern` | `rule[].path_glob` |
    | `wildcard[].startup_command` | `rule[].startup` |
    | `wildcard[].preview_command` | `rule[].preview` |
    | `wildcard[].disable_startup_command` | `rule[].disable_startup` |
    | `wildcard[].windows` | `rule[].tabs` |
