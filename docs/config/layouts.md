---
icon: lucide/panels-top-left
---

# Layouts

!!! warning "Layout settings have no effect in rex-sesh"

    rex-sesh accepts the startup commands, tabs and pane layouts that
    herdr-sesh used, so an older config still loads. It never runs or builds
    them. Choosing a directory creates a Rex session with one terminal that
    starts in that directory, and nothing else. Choosing an open session only
    switches to it.

These settings came from herdr-sesh:

- `startup` in `[workspace_defaults]`, `[[workspace]]` and `[[rule]]`
- `disable_startup` in `[[workspace]]` and `[[rule]]`
- `tabs` in `[[workspace]]` and `[[rule]]`
- the `[[tab]]` and `[[tab.pane]]` tables

Two things about them still matter:

- **They are still checked.** A mistake in one of these tables, such as a pane
  that splits from an unknown pane, stops the config from loading, and the
  picker with it. Run `rex-sesh config validate` after you edit them, or
  remove the tables.
- **Some show up in `rex-sesh list --json`.** Rows carry `startup_command`,
  `disable_startup_command` and `window_names` (the tab names), so scripts that
  read that output can still use them.

The rest of this page lists what each table accepts, so you can keep an older
config valid. See [Configuration](../config.md) for the settings that work.

## `[[tab]]`

| Field | Accepted value |
| --- | --- |
| `name` | Required. Must be unique. `tabs` lists refer to it. |
| `path` | Optional directory. |
| `startup` | Optional command. Can't be combined with `[[tab.pane]]` entries. |
| `pane` | Optional `[[tab.pane]]` entries. See below. |

Every name in a `[[workspace]]` or `[[rule]]` `tabs` list must match a
`[[tab]]`.

## `[[tab.pane]]`

| Field | Accepted value |
| --- | --- |
| `name` | Required. Must be unique within its tab. |
| `split_from` | The name of an earlier pane in the same tab. Required for every pane after the first. |
| `split` | `"right"` or `"down"`. Required for every pane after the first. |
| `ratio` | Optional number from `0.1` to `0.9`. |
| `path` | Optional directory. |
| `env` | Optional table of variables. Names must match `[A-Za-z_][A-Za-z0-9_]*`, and values must not contain NUL bytes. |
| `startup` | Optional command. |
| `wait_for` | Optional `{ match = "...", timeout_ms = 30000 }`. Needs `startup`. `match` must be one non-empty line that doesn't appear in `startup`. `timeout_ms` runs from `1` to `600000`. |
| `focus` | Optional `true`. A tab can focus only one pane, and a `tabs` list can reach only one focused pane. |

The first pane can't set `split_from`, `split` or `ratio`.

## Legacy names

A legacy Sesh-style file uses `[[window]]` for `[[tab]]`, with
`startup_script` for `startup`, and `windows` for `tabs`. These load the same
way and have no effect either. The legacy format has no pane layouts. See the
[key reference](../config.md#legacy-migration) for every renamed key.
