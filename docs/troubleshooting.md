---
icon: lucide/wrench
---

# Troubleshooting

## Check the setup

Run these in a Rex terminal on the machine where the problem happens. Here,
`rex-sesh` means the binary you built, `bin/rex-sesh` in your checkout, or the
copy on a host:

```bash
rex actions | grep sesh_picker
rex keymap | grep sesh_picker
rex-sesh --version
rex-sesh config path
rex-sesh picker --json
```

- `rex actions` should list `sesh_picker`, titled **Sesh: Pick Session**. If
  it doesn't, check that `init.lua` has `require("sesh")`, then run
  `rex config check && rex config reload`.
- `rex keymap` shows the key bound to the picker, if you passed `--key` to
  `rex-sesh lua`.
- `rex-sesh picker --json` prints the rows the picker would show, without
  opening it or switching anything.

`~/.config/rex/sesh.lua` holds the full path of the binary that printed it.
If you move the checkout, print the module again with `rex-sesh lua` and reload
the Rex config. See [Rex app](rex-app.md) for the full setup.

## The picker stays open and says "Could not switch"

When a switch fails, the picker stays open and shows `Could not switch: `
followed by the reason. Fix the cause, then press ++enter++ again.

The most common cause is **Remote Control**. rex-sesh switches sessions
through the Rex app, and the app accepts that request from other programs only
while Remote Control is on. While it is off, the app refuses the switch, and
the picker shows the app's message. Turn on Remote Control in the Rex app's
settings, then press ++enter++ again.

On a directory row, rex-sesh creates the Rex session before it asks the app to
switch. If the switch fails, the picker remembers that session, so a retry
selects it instead of creating another. If you close the picker instead, the
new session stays open in Rex.

## "Sesh cannot start" in a host's session

Rex runs the picker on the machine that holds the current session. In a
session on another host, the picker needs a copy of rex-sesh on that host, at
`~/.local/bin/rex-sesh` or on its `PATH`. A Windows host needs
`%LOCALAPPDATA%\Programs\rex-sesh\rex-sesh.exe`, or `rex-sesh.exe` on its
`PATH`. Without one, the layer shows:

```text
Sesh cannot start: rex-sesh is not installed on this machine.
On this machine, rebuild it with: just build
On another host, install it from your main machine with: just install-host SSH_DESTINATION
```

Rex keeps the layer open with the message and marks its command as exited.
Close the layer with its × button.

Install a copy from your checkout. The argument is an SSH destination, such
as `user@hostname` or a name from `~/.ssh/config`, not the label `rex hosts`
shows. `ssh` must be able to connect to it:

```bash
just install-host devbox
```

The recipe builds rex-sesh for the host's OS and CPU, installs it, and prints
the installed version. Run it again after you update rex-sesh, or the host
keeps its older copy. To install a release on Windows without a checkout, see
[Install on a Windows host](rex-app.md#install-on-a-windows-host).

On a Windows host, a module printed by an older rex-sesh fails before it can
show this message, because it runs `/bin/sh`. Print the module again with
`rex-sesh lua` and run `rex config reload`.

## The picker doesn't open after `just clean`

`just clean` deletes `bin/rex-sesh`, which `sesh.lua` points at. The picker's
start script then looks for `~/.local/bin/rex-sesh` or `rex-sesh` on `PATH`,
so an older copy may open instead. With neither, the layer shows the same
"Sesh cannot start" message as above. Rebuild the binary:

```bash
just build
```

## Rex is not running

rex-sesh never starts a Rex server. If this machine's server doesn't answer,
`rex-sesh picker` stops before it opens, with an error that starts with
`list Rex sessions:` followed by the message from `rex`. `rex-sesh list`
prints a warning that starts with `warning: rex sessions unavailable:`, then
lists your other rows.

Check that the server answers without starting a new one:

```bash
rex --autostart=false ls
```

If you set `REX_SESH_REX_BIN`, check that it names a working `rex` binary.

## Several Rex apps are connected

rex-sesh switches one Rex app. When the picker isn't told which, it uses
`REX_CLIENT`, then the only connected app. With several apps connected, it
tries the only app attached to the picker's session. If that doesn't settle
it, the switch fails with:

```text
N Rex apps are connected; choose one with --client
```

`N` is the number of apps. In the picker, the message follows
`Could not switch: `.

A picker opened from your key or the command palette is told which app to
switch, so this happens only when it isn't:

- You ran `rex-sesh picker` yourself. Pass `--client` with an ID or label from
  `rex client ls`, or set `REX_CLIENT`.
- The picker runs in a session on another host. The host's copy can't use this
  machine's app ID, so it looks for the app itself. Close the other apps, or
  run `rex-sesh picker --client ID` in a terminal on that host.

Without any app, the message is `no Rex app is connected to switch`.

Switching to a host's session from this machine's picker finds the app on that
host. It uses the only app connected to the host, or the one with the same
device name as this machine's app. Otherwise the switch fails with
`no Rex app is connected to HOST` or
`N Rex apps are connected to HOST, and none is this machine's app`.

## A host's sessions are missing

The picker asks every host that `rex hosts` lists at the same time, and waits
up to 3 seconds for them. A host that doesn't answer in time is left out, and
the picker doesn't say so. To see why, run:

```bash
rex-sesh list
```

`list` and `rex-sesh picker --json` print a warning for each host that didn't
answer, starting with `warning: rex host NAME unavailable:`. If `rex hosts`
itself fails, the warning starts with `warning: rex hosts unavailable:`.

## `f2` doesn't open the settings editor

The Rex app handles keys that your Rex config binds before the picker sees
them. Run `rex keymap` and look for `f2`. If your `init.lua` binds it, the
picker never receives it. Edit settings with:

```bash
rex-sesh config edit
```

Or remove the `f2` binding from `init.lua`, then run `rex config reload`. Other
picker keys work the same way: a key your Rex config binds never reaches the
picker.

## The list shakes when you press a key

The picker shakes the list when it refuses a key:

- ++ctrl+x++ would close a session. rex-sesh doesn't support closing sessions.
- ++enter++ while a switch is still running. The footer reads
  `Switching to NAME…`, or `Opening NAME…` for a directory.

## A directory no longer exists

If a zoxide directory was deleted, choosing it fails with
`Could not switch: PATH is not a directory on this machine`, and no session is
created. Remove it from zoxide:

```bash
zoxide remove PATH
```

## A session or directory is missing

- Clear the filter with ++ctrl+u++.
- Run `rex-sesh picker --json` to see every row the picker lists.
- Check `list.blacklist` with `rex-sesh list --blacklisted`, which shows only
  the hidden rows.
- A directory named like an open session on this machine is left out; you see
  the session. See [Sources](config.md#sources).
- Directories need `zoxide` on `PATH` and entries in its database. Check with
  `zoxide query -l`.
- The picker doesn't list `[[workspace]]` entries. Only `rex-sesh list` shows
  them.

## The legacy config warning

rex-sesh prints this when it loads a Sesh-style file:

```text
warning: PATH uses the deprecated Sesh-compatible schema; run 'rex-sesh config migrate' to convert it (see docs/config.md)
```

The file still works. To convert it, run `rex-sesh config migrate`. If the
conversion fails on a `sort_order` that names `tmux`, replace `tmux` with
`rex` and run it again. See [Legacy migration](config.md#legacy-migration).

## Configuration changes have no effect

- Run `rex-sesh config path` and `rex-sesh config validate` in a Rex terminal
  on the same machine. See the [lookup order](config.md#configuration-file-lookup).
- The picker that Rex opens finds its config through the lookup order. A
  picker in a host's session reads that host's config.
- Reopen the picker. It reads its config when it opens.
- Startup commands, tabs and pane layouts have no effect in rex-sesh. See
  [Layouts](config/layouts.md).
- Keys that open the picker belong in your Rex config. Keys inside the picker,
  such as `keys.cycle_preview_mode`, belong in the rex-sesh config.

## Preview is unavailable or shows an error

- The default command preview runs `eza`. Install it, or set
  `workspace_defaults.preview` to another command, such as `ls -la {}`. Test a
  command with `rex-sesh preview TARGET`. Preview commands stop after five
  seconds.
- In pane mode, a session with no terminal to show reads
  `this session has no terminal to preview`. Press ++ctrl+o++, or your
  `keys.cycle_preview_mode` key, to go back to the command preview.
- A session on another host shows its name, host, ID and path in command mode,
  because its directory isn't on this machine.
- If no preview appears, check `picker.show_preview`. In a narrow layer, the
  preview goes below the list.

## Icons or animation look wrong

Source icons need a Nerd Font in the Rex app. Set `picker.show_icons = false`
to show `[rex]` and `[zoxide]` labels instead. The default `eza` preview prints
its own icons; change `workspace_defaults.preview` if they show as boxes.

Set `REX_SESH_REDUCE_MOTION=1` to turn off the picker's animations. See
[Cursor and status indicators](config/picker.md#cursor-and-status-indicators).

## Report a problem

Include:

- the output of `rex-sesh --version` and `rex version`, and your OS
- whether you used the native picker or `--fzf`
- how you opened it: a key, the command palette, or `rex-sesh picker`
- whether the session was on this machine or on a host
- what you expected and what happened, with any message the picker showed
- a minimal config that shows the problem, with private paths and commands
  removed

A screenshot helps with layout problems. Include the layer's size and whether
the Rex app uses a Nerd Font.
