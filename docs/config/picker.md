---
icon: lucide/search
---

# Picker configuration

The picker reads the `[picker]` and `[keys]` tables each time it opens. Change
them in the [Settings editor](../config.md#settings-editor), or in
`config.toml`. For example, add this to a native file that starts with
`version = 1`. If the file already has a `[picker]` table, edit its values
instead of adding another:

```toml
[picker]
show_icons = true
workspace_sort = "agent"
preview_mode = "pane"
```

## `[picker]`

Picker settings change only what the picker shows. They don't change the JSON
that `rex-sesh list --json` or `rex-sesh picker --json` prints.

| Setting | Default | Effect |
| --- | --- | --- |
| `show_icons` | `false` | Show Nerd Font icons in the source column. When off, the column shows `[rex]` or `[zoxide]`. |
| `show_path` | `true` | Show the path column when there is room. Paths still match your filter when the column is hidden. |
| `show_preview` | `true` | Show the preview. `false` gives the list the whole width, runs no preview commands, and turns off the preview key. |
| `preview_mode` | `"command"` | The preview the picker opens with: `command` or `pane`. See [Preview controls](#preview-controls). |
| `prioritize_home` | `true` | Put your home directory first when the filter is exactly `home`. See [Search ranking](#search-ranking). |
| `separator_aware` | `false` | Treat `-`, `_`, `/` and `.` as spaces when matching your filter. |
| `prompt` | `"Sesh> "` | The filter prompt. Empty uses the default. |
| `placeholder` | `"Filter sessions"` | The hint shown in an empty filter. Empty uses the default. |
| `workspace_sort` | `"workspace"` | How Rex sessions are ordered when the picker opens: `workspace`, `recent`, or `agent`. See [Sort modes](#sort-modes). |
| `herdr_theme_inherit` | `true` | Take the picker's colors from Herdr's theme file. `false` uses the built-in colors. See [Picker colors](#picker-colors). |
| `replace_worktree_icon` | `true` | Accepted, but has no effect. rex-sesh doesn't detect Git worktrees. |
| `show_last_workspace` | `true` | Show the previous open session in the native footer when space allows. |
| `show_last_workspace_path` | `true` | Include its directory when the footer has room. |

The keys keep their herdr-sesh names, such as `workspace_sort`, so older
configs still load.

### Sort modes

Press ++ctrl+r++ in the picker to cycle through the modes: `workspace`, then
`recent`, then `agent`. The footer names the current mode.

| Mode | Footer | Order |
| --- | --- | --- |
| `workspace` | `rex order` | The order Rex lists sessions in: this machine's, then each host's. |
| `recent` | `recent` | The sessions the picker last switched this app to, newest first, after the session the picker is in when you opened it with a key or the palette. Sessions it never switched to follow in Rex order. See [Session history](../rex-app.md#session-history). |
| `agent` | `agent` | By the most urgent status in each session: blocked, then done, then working, then idle, then sessions with no status. Sessions with the same status keep Rex order. |

In `agent` mode, the picker sorts again each second as statuses change, and
keeps your selection on the same row. Sorting moves only Rex sessions.
Directories stay where [`list.source_order`](../config.md#sources) puts them.

### Search ranking

The filter matches names and paths, ignoring case, wherever the text appears.
Name matches come first, then rows that match only by path. Each group keeps
its order.

A host's session also matches its host's name: `devbox api` finds
`[devbox] api`.

The filter `home`, exactly, also finds your home directory, whatever its row is
called. With `prioritize_home = true` that row comes first; with `false` it
stays among the path matches. Rows from other hosts never match this way.

The fzf picker (`rex-sesh picker --fzf`) does its own fuzzy matching. It uses
`prompt` and `placeholder`, but `separator_aware` and `prioritize_home` have no
effect there.

### Preview controls

`command` mode runs a preview command for the selected row. The command comes
from a [`[[rule]]`](../config.md#rule), from
[`workspace_defaults.preview`](../config.md#workspace_defaults), or from the
built-in default, `eza --icons=always --color=always -la {}`. For a Rex
session, `{}` is the directory its focused terminal last reported. A session
with no directory shows its name and ID instead. A session on another host
shows its name, host, ID and path, because its directory isn't on this
machine.

`pane` mode shows the selected session's live screen: the focused terminal of
its active tab. It refreshes once a second, and shows only what is on screen,
not the scrollback. For a host's session, the
picker reads the screen through that host. A directory has no screen, so it
keeps its command preview, and the preview title says PREVIEW instead of PANE.

Press ++ctrl+o++ to switch between the two modes while the picker is open. The
preview title shows the key. See [`[keys]`](#keys) to change it.

- `show_preview = false` turns off both modes and the key.
- A preview command that runs longer than five seconds is stopped. Moving to
  another row cancels the old preview.
- `preview_mode` doesn't change `rex-sesh preview`, which always runs the
  preview command.
- With `show_path = false`, a side-by-side preview takes half the width. In a
  narrow layer, the preview goes below the list.
- The fzf picker shows its own preview, a README through `bat` or a file list,
  and ignores preview commands.

Drag the divider between the list and the preview to resize it. The new width
lasts until the picker closes. See [Keybindings](../keybindings.md) for the
picker's other keys.

## `[keys]`

```toml
[keys]
cycle_preview_mode = "alt+p" # default: "ctrl+o"; "" turns the key off
```

`cycle_preview_mode` is the key that switches between command and pane
previews. It takes precedence over the picker's other keys, so choose one the
picker doesn't use. An empty value turns the key off and hides its hint;
`preview_mode` still sets the preview the picker opens with.

If you set it to `"f2"`, the settings editor moves to `ctrl+,`. The Rex app
takes any key your Rex config binds before the picker sees it, so check
`rex keymap` before you choose one. This table doesn't configure fzf.

??? info "Key syntax"

    Names use Bubble Tea's exact, case-sensitive spelling: a printable
    character, or a named key such as `f2`, `tab` or `pgdown`. Join modifiers
    in `ctrl`, `alt`, `shift`, `meta`, `hyper`, `super` order. Unsupported
    names, repeated modifiers and the wrong order are rejected when the config
    loads.

    For a shifted printable key alone, use the character it types: `P` or
    `?`, not `shift+p` or `shift+/`. With other modifiers, use the unshifted
    key: `ctrl+shift+p` and `alt+shift+/`, not `ctrl+shift+P` or
    `alt+shift+?`. Named keys such as `shift+f2` are valid.

## Picker colors

rex-sesh doesn't read the Rex app's theme. With `herdr_theme_inherit = true`,
the default, it reads the theme from Herdr's config file, the first of:

1. `HERDR_CONFIG_PATH`
2. `$XDG_CONFIG_HOME/herdr/config.toml`
3. `~/.config/herdr/config.toml`

Without that file, the picker uses the `catppuccin` theme. With
`herdr_theme_inherit = false`, it uses its built-in colors. The settings
editor uses the same colors.

To pick a theme without Herdr, create the file, for example
`~/.config/herdr/config.toml`:

```toml
[theme]
name = "tokyo-night"

[theme.custom]
accent = "#7dcfff"
```

??? info "Light and dark themes"

    With `[theme] auto_switch = true`, the picker checks the terminal's
    background when it opens, and uses `light_name` or `dark_name`, with
    `[theme.custom.light]` or `[theme.custom.dark]`. If the background can't
    be read, it uses the dark theme.

    An omitted mode name comes from `name`: `tokyo-night` uses `tokyo-night`
    on a dark background and `tokyo-night-day` on a light one. A theme
    without a pair, such as `nord`, is used for both. An empty or unknown mode
    name uses `catppuccin` for dark and `catppuccin-latte` for light.

??? info "Themes and color tokens"

    Themes: `catppuccin` (the default), `catppuccin-latte`, `tokyo-night`,
    `tokyo-night-day`, `dracula`, `nord`, `gruvbox`, `gruvbox-light`,
    `one-dark`, `one-light`, `solarized`, `solarized-light`, `kanagawa`,
    `kanagawa-lotus`, `rose-pine`, `rose-pine-dawn`, and `vesper`. Common
    aliases such as `catppuccin-mocha`, `tokyonight` and `onedark` work too.

    | Token | Picker role |
    | --- | --- |
    | `accent` | Prompt, cursor, selection rail, the `◆` current-session mark, done (`●`), and the focused box's border |
    | `mauve` | Title, section headings, filter matches, and the cursor trail |
    | `text` | Row names |
    | `subtext0` | Paths, counts, help text, and other borders |
    | `green` | Idle (`✓`) |
    | `yellow` | Working (spinner), the empty-list message, and error messages |
    | `red` | Blocked (`◉`) |
    | `overlay1` | The `ghost` cursor trail |

    A `[theme.custom]` value must be a `#RGB` or `#RRGGBB` color. Other values
    are skipped, so a partial table changes only the roles it sets. The
    `terminal` theme has no fixed colors, so the picker keeps its built-in
    colors there unless you add overrides.

## Cursor and status indicators

The picker reads these environment variables when it starts.
`REX_SESH_SMEAR_PRESET` picks the cursor animation:

| Preset | Effect |
| --- | --- |
| `crisp` (default) | A fast rail with a short line trail. |
| `gooey` | A slower block cursor with a long shaded trail and eased movement. |
| `ghost` | A soft diamond cursor with a dotted, low-contrast trail. |

Set `REX_SESH_REDUCE_MOTION=1` (or `true`) to turn off the cursor trail and the
picker's other animations. The working spinner then stands still.

Each session row shows the most urgent status that programs in the session
report to Rex. The picker refreshes statuses once a second.

| Glyph | Status |
| --- | --- |
| Spinner | Working |
| `◉` | Blocked |
| `●` | Done |
| `✓` | Idle |
| None | No program reports a status |

A program that reports an error counts as idle. `◆` marks the session the
picker was opened from; on the selected row, the selection rail takes its
place.
