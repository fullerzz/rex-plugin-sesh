# Rex setup

rex-sesh adds one action to Rex, `sesh_picker`. The command palette lists it as
**Sesh: Pick Session**. It opens the picker in a floating layer over the current
tab. When you choose a row, the picker asks the Rex app to switch to that
session, and the layer closes.

You add the action to Rex with a Lua module, `sesh.lua`, that rex-sesh prints
for you.

## Install

1. Build rex-sesh in your checkout:

    ```bash
    mise install
    just build
    ```

2. Write the module beside your Rex config. `--key` also binds a key to the
   picker. Keep the quotes: without them, the shell reads `>` as a redirect.

    ```bash
    ./bin/rex-sesh lua --key 'ctrl+a>s' > ~/.config/rex/sesh.lua
    ```

3. Load the module from `~/.config/rex/init.lua` by adding this line:

    ```lua
    require("sesh")
    ```

4. Check the config, then make the Rex server read it again:

    ```bash
    rex config check && rex config reload
    ```

    `rex config check` loads the file and reports any problem in it. It exits
    with status 1 when a problem is an error.

5. Turn on **Remote Control** in the Rex app's settings. See
   [Remote Control](#remote-control).

To check the result, run `rex actions`. It lists `sesh_picker` with the title
`Sesh: Pick Session` and the optional argument `focus`. `rex keymap` shows the
key bound to `sesh_picker`.

## What the module does

`rex-sesh lua` prints the module for the binary that runs it. This is the
output of `rex-sesh lua --key 'ctrl+a>s'`, shortened:

```lua
local bin = "/path/to/rex-plugin-sesh/bin/rex-sesh"

rex.action{
  name = "sesh_picker",
  title = "Sesh: Pick Session",
  -- ...
  args = { focus = "boolean?" },
  run = function(ctx, args)
    -- ...
    local argv = { "/bin/sh", "-c", start, bin, "picker" }
    if ctx.client_id and (ctx.origin == "key" or ctx.origin == "palette") then
      argv[#argv + 1] = "--opener"
      argv[#argv + 1] = ctx.client_id
    end
    return assert(rex.session.new_layer{
      window_id = window,
      bounds = { 0.1, 0.1, 0.8, 0.8 },
      focus = not (args and args.focus == false),
      -- ...
    })
  end,
}

rex.bind("ctrl+a>s", "sesh_picker")
```

- `bin` is the full path of the rex-sesh binary that printed the module. A
  symlink is resolved to the file it points to.
- The action needs a focused terminal in a tab. Without one, it fails with
  "Sesh needs a focused terminal" or "The terminal must be in a tab".
- It opens a floating layer over the tab that holds that terminal. The layer
  runs `rex-sesh picker` through a short start script, `start`. See
  [Open the picker on a host](#open-the-picker-on-a-host).
- The picker finds the session it was opened from through the terminal's
  `REX_BLOCK` variable, and marks that session with `◆`.
- When you open the picker from a key or the command palette, the action
  passes `--opener` with the ID of the Rex app you used. That app is the one
  that switches, and it shows the session the picker is in. When something
  else runs the action, such as `rex do`, it passes no `--opener`, and the
  picker finds the app itself. See [Commands](commands.md) for how `picker`
  chooses an app.
- The `focus` argument decides whether the layer takes focus. It does unless
  you pass `focus=false`.
- Without `--key`, the module ends with a commented example in place of
  `rex.bind`.

## Bind a key

The quickest way is `--key` when you write the module. The key uses Rex's key
syntax, so `ctrl+a>s` means ++ctrl+a++, then ++s++.

To keep the binding in your own config instead, write the module without
`--key`, and bind the action in `init.lua` after the `require` line:

```lua
require("sesh")
rex.bind("ctrl+a>s", "sesh_picker")
```

Then run `rex config check && rex config reload` again.

If your Rex config binds a key that the picker uses, the Rex app takes the key
first, and the picker never sees it. For example, a config that binds ++f2++
hides the picker's settings key. Run `rex-sesh config edit` to open the
settings editor instead. `rex keymap` lists what each key does. See
[Keybindings](keybindings.md) for the picker's keys.

## Open the picker without a key

- **Command palette**: run **Sesh: Pick Session**.
- **Command line**: from a terminal in a Rex session, run the action by name:

    ```bash
    rex do sesh_picker
    rex do sesh_picker focus=false
    ```

    The second form opens the picker without moving focus to it.

## Remote Control

The picker switches sessions through the Rex app's `session.select` action.
The app accepts that action from other programs only while **Remote Control**
is on in its settings. Remote Control is off until you turn it on.

If the app refuses a switch, the picker stays open. It shows
"Could not switch:" followed by the app's message. Turn on Remote Control, then
press ++enter++ again.

On a zoxide directory, the picker creates the new Rex session before it asks
the app to switch. If the switch is refused, ++enter++ selects that same
session again instead of creating another. If you close the picker instead,
the new session stays open in Rex.

## Sessions on other hosts

Hosts are the other Rex servers that `rex hosts` lists. The picker lists each
host's sessions after this machine's sessions, in `rex hosts` order. A host's
session shows the host's name first, such as `[devbox] api`.

- The picker asks all hosts at once and waits up to 3 seconds for each. A host
  that fails or doesn't answer in time is left out. The picker says nothing
  about it, but `rex-sesh list` and `rex-sesh picker --json` print a warning
  that names the host.
- Enter on a host's session switches through that host's server. The Rex app
  has a different client ID on each host, so the picker switches the only app
  connected to that host. With several apps connected there, it switches the
  one with the same device name as the app it chose on this machine.
- In pane preview (++ctrl+o++), a host's session shows its live screen, read
  through the host. The command preview runs nothing. It shows the session's
  name, host, ID and path, because that directory isn't on this machine.
- zoxide directories come from this machine. A session that the picker opens
  from a directory starts on this machine's Rex server.

## Open the picker on a host

Rex runs the layer on the machine that holds the current session. When you open
the picker in a host's session, the layer runs on that host, where the path in
`bin` usually doesn't exist. The start script handles this:

```sh
if [ -x "$0" ]; then exec "$0" "$@"; fi
for c in "$HOME/.local/bin/rex-sesh" "$(command -v rex-sesh)"; do
  if [ -n "$c" ] && [ -x "$c" ]; then exec "$c" picker; fi
done
printf '%s\n' "Sesh cannot start: rex-sesh is not installed on this machine." \
  "On this machine, rebuild it with: just build" \
  "On another host, install it from your main machine with: just install-host HOST" >&2
exit 127
```

1. If `bin` exists on the machine, the script runs it with the action's
   arguments.
2. Otherwise it runs `~/.local/bin/rex-sesh`, or `rex-sesh` on `PATH`. It
   leaves out `--opener`, because the app's client ID belongs to this
   machine's server.
3. If neither exists, the layer shows:

    ```text
    Sesh cannot start: rex-sesh is not installed on this machine.
    On this machine, rebuild it with: just build
    On another host, install it from your main machine with: just install-host HOST
    ```

    The layer stays open. Close it with its × button.

The picker on a host works from that host's point of view:

- It lists the host's own sessions, the sessions on the hosts that its own
  `rex hosts` lists, and its own zoxide directories. It lists this machine's
  sessions only if the host lists this machine as one of its hosts.
- It reads its configuration from the host's own config locations.
- It switches the only Rex app connected to that host. With several, it
  switches the only one attached to the session you opened it from. Otherwise
  the switch fails, and the picker shows the reason.

### Install rex-sesh on a host

Run `just install-host` in your checkout, with one or more SSH destinations:

```bash
just install-host devbox lab
```

For each destination, the recipe:

1. Reads the host's OS and CPU with `ssh HOST uname -sm`. It supports Linux
   and macOS, on amd64 or arm64.
2. Builds rex-sesh for that OS and CPU on this machine. The host doesn't need
   Go.
3. Uploads it beside `~/.local/bin/rex-sesh` on the host, then renames it into
   place, so a picker that is running keeps its binary.
4. Prints the installed version.

The arguments are SSH destinations, such as names from `~/.ssh/config`. They
can differ from the host labels that `rex hosts` shows. The recipe runs `ssh`
in batch mode, so it needs a login that doesn't prompt for a password.

A host's copy doesn't change when you rebuild here. Run `just install-host`
again after you update rex-sesh.

## Session history

Rex doesn't report which session an app shows, or the order in which the app
showed sessions. So rex-sesh keeps its own history of the switches the picker
makes. The picker's `recent` order comes from it.

The picker records a switch when the Rex app accepts it, in the native picker
and with `--fzf`. It records the session it switched to. For a directory row,
it records the session that the switch created. When you open the picker with
a key or the command palette, the app you used shows the session the picker
is in, so the picker records that session too, as the one before. Opened
another way, such as `rex-sesh picker --client`, the picker can't tell which
session the app shows, so it records only where the app went.

The picker records nothing when:

- You open the picker, or close it without a switch.
- You open it with a key or the palette, and choose the session it is in.
- The app refuses the switch.

When two pickers switch one app at the same time, the second waits for the
first. So the history keeps the order in which the app took the switches.

Each Rex app has its own history. The picker names an app by its device label,
such as `Studio Mac`, which the app keeps on every host. An app without a
label is named by its client ID. Two apps with the same label share one
history. If the picker can't tell which app it switches, it shows no history.
This happens when several apps are connected and the picker has neither
`--client` nor `--opener`.

A switch to a host's session goes to the app on that host. If that app has
the same label, the switch goes in this app's history. If the host's only app
is another app, the switch goes in that app's history, without the session
that the picker opened in. If that app has no label, the switch isn't
recorded.

The history names each session by its host and its session ID. So sessions
with the same name, or the same ID, on two hosts stay apart. The picker skips
a session that is closed, which means that its server listed its sessions
without it. If a host doesn't answer, its sessions stay in the history. If
the newest of the other sessions in the history is on that host, the picker
can't name the previous session.

The history is `session-history.json` in the state directory: `REX_SESH_STATE_DIR`,
or `$XDG_STATE_HOME/rex-sesh` (by default `~/.local/state/rex-sesh`). Each
machine that runs the picker keeps its own. A picker opened in a host's
session runs on that host, so it reads and writes that host's history. It
runs without `--opener`, so it records only where the app went. See
[Open the picker on a host](#open-the-picker-on-a-host).

### What the history can't see

The picker records only its own switches. Rex doesn't report the other ways to
change the session, so the history leaves these out:

| Change | Recorded |
| --- | --- |
| Choose a session in the picker | Yes |
| Rex's session commands, such as **Change Session…**, **Next Session**, **Previous Session** and **Go to Session** | No |
| **Navigate Back** and **Navigate Forward** | No |
| Change tabs or panes in a session | No. This doesn't change the session. |
| Choose a session in a picker on a host | Only in that host's history |

After a change that the history can't see, the picker names the last session
that it recorded. For example, you switch from A to C with the picker. Then you
choose B with **Change Session…**, and then D. The picker that you open in D
names C, not B.

Rex's own commands for going back work differently. Rex describes
**Navigate Back** as a return to the pane you were in before, in any tab or
session, so it steps back through panes, not sessions. **Previous Session**
and **Next Session** cycle through the session list. Neither command makes
its history readable, from Lua or from the server API.

## Update after a rebuild

The module runs the binary at the path in `bin`. `just build` writes a new
binary to the same path, so the next picker you open runs the new build. You
don't need to write the module again or reload Rex.

Write the module again, and run `rex config reload`, when:

- you move or rename the checkout, because `bin` holds the old path;
- you pull a new version, so the module matches the binary;
- your module passes `--client` instead of `--opener`. A module written before
  `--opener` existed still switches, but the history doesn't record the
  session that you switched from. See [Session history](#session-history).

`just clean` deletes `bin/rex-sesh`. Until you run `just build` again, the
start script falls back to `~/.local/bin/rex-sesh` or `rex-sesh` on `PATH`,
without `--opener`. If neither exists, the layer shows "Sesh cannot start",
even on this machine. Run `just build` to fix it.

## When the picker cannot start

If the picker fails before it opens, for example because of an error in your
rex-sesh config, it exits and prints the error. Rex keeps the layer open with
that output. Close it with its × button. See
[Troubleshooting](troubleshooting.md) for common causes.

## Remove the picker

1. Delete the `require("sesh")` line, and any `rex.bind` for `sesh_picker`,
   from `~/.config/rex/init.lua`.
2. Delete `~/.config/rex/sesh.lua`.
3. Run `rex config reload`.

To remove the copies on your hosts, delete `~/.local/bin/rex-sesh` on each
one. rex-sesh's own config, in `~/.config/rex-sesh`, and its cache, in
`~/.local/state/rex-sesh`, stay until you delete them.
