# Rex setup

The Lua module registers two actions in the Rex command palette.
**Sesh: Pick Session** opens the picker in a floating layer.
**Sesh: Last Session** returns directly to the previous open session
recorded for your app. To bind both, pass their keys when generating the module:

```bash
./bin/rex-sesh lua --key 'ctrl+a>s' --last-key 'ctrl+a>shift+l' > ~/.config/rex/sesh.lua
```

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

`rex-sesh lua` prints the module for the binary that runs it. The picker runs in a layer on the machine holding the session.
**Sesh: Last Session** runs `last` on the server that loaded the module,
including while the app displays a remote session.

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
`bin` usually doesn't exist. On a Linux or macOS host, the start script handles
this:

```sh
if [ -x "$0" ]; then exec "$0" "$@"; fi
for c in "$HOME/.local/bin/rex-sesh" "$(command -v rex-sesh)"; do
  if [ -n "$c" ] && [ -x "$c" ]; then exec "$c" picker; fi
done
printf '%s\n' "Sesh cannot start: rex-sesh is not installed on this machine." \
  "On this machine, rebuild it with: just build" \
  "On another host, install it from your main machine with: just install-host SSH_DESTINATION" >&2
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
    On another host, install it from your main machine with: just install-host SSH_DESTINATION
    ```

    The layer stays open. Close it with its × button.

A Windows host has no `/bin/sh`. Before it opens the layer, the module asks the
host's Rex server for its platform (`server.version`), once per host label
until the config reloads. For a Windows host, the layer runs PowerShell, with
no shell profile, instead of the start script. The script is base64 encoded, so
Windows command-line quoting can't change it. It runs the host's own copy:

1. `%LOCALAPPDATA%\Programs\rex-sesh\rex-sesh.exe`, or `rex-sesh.exe` on
   `PATH`, with `picker` and no `--opener`.
2. If neither exists, the layer says that rex-sesh is not installed, gives the
   install path, and the command exits 127.

If the host doesn't answer the platform request, the layer uses the start
script. When the config itself runs on Windows, the module runs `bin` directly,
with `--opener`, for sessions on that machine.

Rebuild the module with `rex-sesh lua` and run `rex config reload` after you
update rex-sesh. A module printed by an older rex-sesh always uses the start
script, which can't run on Windows.

**Sesh: Last Session** uses a different execution path. The config server runs
the command directly with `rex.exec`. The key and command palette supply
the active session, its host, and the app's client ID on the config server.
For example, after the local picker takes you from local A to devbox/C, the
shortcut in C reads the original history and returns to A. It also works if
devbox has its own rex-sesh history or has no rex-sesh installed. If the
config server uses a nondefault local socket, set `REX_SERVER` in its
process environment to that socket.

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
just install-host devbox jo@winbox
```

The arguments are SSH destinations, such as `user@hostname` or names from
`~/.ssh/config`. They are not the host labels that `rex hosts` shows: the
recipe passes each one to `ssh` and `scp` as given. When `ssh` can't connect,
the recipe stops and says so. The recipe runs `ssh` in batch mode, so it needs
a login that doesn't prompt for a password.

For each destination, the recipe:

1. Reads the host's OS and CPU with `ssh DESTINATION uname -sm`. If that fails
   but SSH connects, it treats the host as Windows and reads
   `PROCESSOR_ARCHITECTURE` with PowerShell. It supports Linux, macOS and
   Windows, on amd64 or arm64.
2. Builds rex-sesh for that OS and CPU on this machine. The host doesn't need
   Go.
3. Uploads it beside the installed copy, then renames it into place, so a
   picker that is running keeps its binary. That is `~/.local/bin/rex-sesh` on
   Linux and macOS, and `%LOCALAPPDATA%\Programs\rex-sesh\rex-sesh.exe` on
   Windows. Windows can't overwrite a running `.exe`, so the old copy becomes
   `rex-sesh.exe.old` beside it, and the next install removes it.
4. Prints the installed version.

A host's copy doesn't change when you rebuild here. Run `just install-host`
again after you update rex-sesh.

### Install on a Windows host

The Windows AMD64 release is a `.zip` that holds `bin/rex-sesh.exe`, and
`checksums.txt` lists it. To install a release from a Mac onto a Windows host
that runs OpenSSH Server, set `dest` to the host's SSH destination, not its Rex
host label:

```bash
tag=vX.Y.Z
version=${tag#v}
dest=user@hostname
gh release download "$tag" --repo fullerzz/rex-plugin-sesh \
  --pattern "rex-sesh_${version}_windows_amd64.zip" --pattern checksums.txt
shasum -a 256 --check --ignore-missing checksums.txt
unzip -o "rex-sesh_${version}_windows_amd64.zip"
ssh "$dest" 'powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path $env:LOCALAPPDATA\Programs\rex-sesh | Out-Null"'
scp "rex-sesh_${version}_windows_amd64/bin/rex-sesh.exe" "$dest:AppData/Local/Programs/rex-sesh/rex-sesh.exe.new"
ssh "$dest" 'powershell -NoProfile -Command "Remove-Item -Force -ErrorAction SilentlyContinue $env:LOCALAPPDATA\Programs\rex-sesh\rex-sesh.exe.old; if (Test-Path $env:LOCALAPPDATA\Programs\rex-sesh\rex-sesh.exe) { Move-Item -Force $env:LOCALAPPDATA\Programs\rex-sesh\rex-sesh.exe $env:LOCALAPPDATA\Programs\rex-sesh\rex-sesh.exe.old }; Move-Item $env:LOCALAPPDATA\Programs\rex-sesh\rex-sesh.exe.new $env:LOCALAPPDATA\Programs\rex-sesh\rex-sesh.exe; & $env:LOCALAPPDATA\Programs\rex-sesh\rex-sesh.exe --version"'
```

The copy uploads beside the installed one as `rex-sesh.exe.new`. Only after
the upload succeeds does the installed copy move aside to `rex-sesh.exe.old`,
since Windows can rename a running `.exe` but not overwrite it. An interrupted
upload leaves the installed copy working.

These commands make some assumptions:

- The SSH server starts `cmd.exe`, which is Windows OpenSSH's default. If its
  default shell is PowerShell, or the account's path has a space, use
  `just install-host`, which works with either shell.
- `%LOCALAPPDATA%` is the default `AppData\Local` in the account's profile,
  because `scp` resolves a relative path from the profile directory.

Then, on the Mac, rebuild the config module so it knows how to start the picker
on Windows, and reload Rex:

```bash
~/.local/bin/rex-sesh lua --key 'ctrl+a>s' > ~/.config/rex/sesh.lua
rex config reload
```

On the Windows host, check that the copy lists the host's sessions:

```powershell
& "$env:LOCALAPPDATA\Programs\rex-sesh\rex-sesh.exe" list --json
```

On Windows, rex-sesh keeps its state in `%USERPROFILE%\.local\state\rex-sesh`,
or in `XDG_STATE_HOME` or `REX_SESH_STATE_DIR` when they are set. It runs
preview commands with `cmd.exe /d /s /c`. In a preview command, `{}` becomes
`"%REX_SESH_PREVIEW_PATH%"`, and rex-sesh sets that variable to the path, so
`cmd.exe` keeps `%` and `&` in the path literal. The
native picker works on Windows. `--fzf` does not, because its preview script
needs `sh`.

## Session history

Rex doesn't report which session an app shows, or the order in which the app
showed sessions. So rex-sesh keeps its own history of accepted picker and last-session switches. The picker's `recent` order comes from it.

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

Each Rex app has its own history, keyed by its client ID on the machine
running the picker. Two connected apps can have the same device label and
keep independent visit orders. If the app reconnects with a new client ID,
for example after a server restart, its recent order starts over. Earlier
visits stay on disk under the old ID. If the picker can't identify the local app,
it shows no history for it.

For a remote switch, Rex assigns the app a different ID on the host. The
picker records the switch when the accepting app's device label identifies
one app on each server. If that identifies a different local app, its
history records only the destination.

The history names each session by its host and its session ID. So sessions
with the same name, or the same ID, on two hosts stay apart. The picker skips
a session that is closed, which means that its server listed its sessions
without it. If a host doesn't answer, its sessions stay in the history. If
the newest of the other sessions in the history is on that host, the picker
can't name the previous session.

The history is `session-history.json` in the state directory: `REX_SESH_STATE_DIR`,
or `$XDG_STATE_HOME/rex-sesh` (by default `~/.local/state/rex-sesh`). Each
machine that runs the picker keeps its own. A picker opened in a host's
session runs on that host, so it reads and writes that host's history.
When you use the shortcut, it reads the config server's history. To
use visits recorded by a remote picker, run `rex-sesh last` in a terminal
on that host. See
[Open the picker on a host](#open-the-picker-on-a-host).

### What the history can't see

The picker records only its own switches. Rex doesn't report the other ways to
change the session, so the history leaves these out:

| Change | Recorded |
| --- | --- |
| Choose a session in the picker or invoke **Sesh: Last Session** | Yes |
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
