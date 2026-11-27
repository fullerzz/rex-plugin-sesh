---
icon: lucide/panels-top-left
---

# rex-sesh

A [Sesh](https://github.com/joshmedeski/sesh)-style session picker for the Rex
terminal.

Press a key in Rex, and the picker opens in a floating layer over the current
tab. Type to filter, press ++enter++, and the Rex app switches to the session
you chose.

The picker lists, in this order:

1. The open Rex sessions on this machine.
2. The sessions on each of your hosts, the other Rex servers that `rex hosts`
   lists. They show the host's name first, such as `[devbox] api`.
3. Your [zoxide](https://github.com/ajeetdsouza/zoxide) directories.

Choosing a session switches the Rex app to it. Choosing a directory creates a
Rex session named after it, starting in it, and then switches to it.

## Requirements

- macOS or Linux.
- The Rex app, with **Remote Control** turned on in its settings. rex-sesh
  switches sessions through the app's `session.select` action, and the app
  accepts actions from other programs only while Remote Control is on.
- Git, to get the source, and [mise](https://mise.jdx.dev), to install the
  pinned Go toolchain, `just` and the other tools.
- Optional: `zoxide`, to list your frequent directories.
- Optional: `eza`, for the default directory preview.
- Optional: `fzf` and `bat`, for the fzf picker and its preview.

## Build

```bash
git clone https://github.com/fullerzz/rex-plugin-sesh.git
cd rex-plugin-sesh
mise install
just build
```

The binary is `bin/rex-sesh`. Check it with:

```bash
./bin/rex-sesh --version
./bin/rex-sesh picker --json
```

`picker --json` prints the picker's rows as JSON and switches nothing.

## Set up in Rex

1. Write the Rex config module, and bind a key to the picker:

    ```bash
    ./bin/rex-sesh lua --key 'ctrl+a>s' > ~/.config/rex/sesh.lua
    ```

2. Add this line to `~/.config/rex/init.lua`:

    ```lua
    require("sesh")
    ```

3. Check the config, then reload it:

    ```bash
    rex config check && rex config reload
    ```

4. Turn on Remote Control in the Rex app's settings.

The module points at `bin/rex-sesh` in this checkout. `just clean` deletes that
binary, so run `just build` again before you use the picker.

[Rex setup](rex-app.md) explains what the module does, other ways to bind a
key, sessions on other hosts, and how to remove the picker.

## First use

Press ++ctrl+a++, then ++s++. You can also run **Sesh: Pick Session** from the
Rex command palette.

You see your Rex sessions, with `◆` on the session you opened the picker from.
Then come your hosts' sessions and your zoxide directories. Beside each
session, a glyph shows the most urgent status that its programs report through
Rex:

| Glyph | Status |
| --- | --- |
| Spinner | Working |
| `◉` | Blocked |
| `●` | Done |
| `✓` | Idle |

Type part of a name or path to filter the list. Press ++enter++ to switch to the
selected session. On a directory, ++enter++ opens a new Rex session there,
switches to it, and adds the directory to zoxide. Press ++esc++ to close the
picker without switching.

If the Rex app refuses the switch, for example because Remote Control is off,
the picker stays open and shows "Could not switch:" with the app's message.

No config file is needed. To change how the picker looks, what its previews
run, or which entries it hides, see [Configuration](config.md). The picker
doesn't list `[[workspace]]` entries from your config; `rex-sesh list` does.

### Everyday tasks

- [Bind another key to the picker](rex-app.md#bind-a-key).
- [Use the picker with sessions on other hosts](rex-app.md#sessions-on-other-hosts).
- [Learn the picker's keys and switch previews](keybindings.md).
- [Change the picker's appearance and previews](config/picker.md).
- [Run rex-sesh commands directly](commands.md).
- [Diagnose a missing session or an unexpected result](troubleshooting.md).

### Reference

- [Rex setup](rex-app.md) covers the `sesh_picker` action, Remote Control,
  hosts, updates and removal.
- [Configuration](config.md) explains config lookup, settings and legacy Sesh
  migration. [Picker](config/picker.md) covers the picker's settings.
- [Commands](commands.md) lists every rex-sesh command and flag.
- [Keybindings](keybindings.md) lists the keys of the picker and the settings
  editor.
- [Contributing](development/index.md) covers local development, tests and
  docs.
