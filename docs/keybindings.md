---
icon: lucide/keyboard
---

# Keybindings

This page lists the keys inside the picker and the settings editor. To bind the
key that opens the picker, see [Rex setup](rex-app.md#bind-a-key).

The picker runs in a Rex terminal, so the Rex app sees each key first. If your
Rex config binds a key that the picker uses, the app takes it, and the picker
never sees it. For example, a config that binds ++f2++ hides the picker's
settings key. Run `rex-sesh config edit` to open the settings editor instead.

## Native picker controls

| Key | Action |
| --- | --- |
| ++enter++ | Switch the Rex app to the selected session. On a directory, create a Rex session there, then switch to it. The picker closes once the switch succeeds. |
| ++esc++ / ++ctrl+c++ | Close the picker without switching. |
| ++down++ / ++ctrl+n++ / ++ctrl+j++ | Move from the filter into the list, then move down. |
| ++up++ / ++ctrl+p++ / ++ctrl+k++ | Move up in the list. Above the first row, return to the filter. In the filter, ++ctrl+k++ keeps its text-editing behavior. |
| ++right++ | Return from the list to the filter. In the filter, move the text cursor. |
| ++ctrl+u++ | Clear the filter and focus it. |
| ++ctrl+v++ | Paste into the filter. |
| ++ctrl+r++ | Cycle the order of the Rex sessions: `rex order`, `recent`, `agent`. |
| ++ctrl+o++ | Switch between the command preview and the pane preview. This is the default of [`keys.cycle_preview_mode`](config/picker.md), which changes or turns off the key. |
| ++f2++ | Open the settings editor. When `keys.cycle_preview_mode` is `f2`, this key is **Ctrl+,** instead. |
| ++ctrl+x++ | Not supported. rex-sesh can't close sessions, so the picker refuses the key. |

Typing updates the filter, even when the list is focused. Any key that isn't in
the table goes to the filter, and focus moves back to it.

When the picker refuses a key, the list shakes, unless `REX_SESH_REDUCE_MOTION`
is set.

The footer lists the main keys and the current order, for example
`f2 settings · enter select · ctrl+j/k · ctrl+r rex order · esc exit`.

### While a switch runs

After ++enter++, the footer reads "Switching to NAME…", or "Opening NAME…" for a
directory. Until the switch ends, the picker refuses another ++enter++ or a
click on a row, and the settings key does nothing.

If the Rex app refuses the switch, the picker stays open and shows
"Could not switch:" with the app's message. The next key you press hides the
message. Press ++enter++ to try again. See
[Remote Control](rex-app.md#remote-control).

### Session order

++ctrl+r++ changes the order of the Rex sessions. Directories stay after them.
The footer names the current order.

| Order | Effect |
| --- | --- |
| `rex order` | This machine's sessions, then each host's sessions, as Rex lists them. |
| `recent` | The sessions the picker last switched this app to, newest first, after the session the picker is in when you opened it with a key or the palette. The rest follow in `rex order`. See [Session history](rex-app.md#session-history). |
| `agent` | Sessions by status: blocked, done, working, idle, then sessions with no status. Hosts' sessions mix with this machine's. |

The order goes back to the configured one each time the picker opens. See
[Picker configuration](config/picker.md) to change it.

### Mouse

- Left-click a row to choose it, as ++enter++ does.
- The wheel moves the selection, as the arrow keys do. Over the preview box,
  it scrolls the preview.
- When the list has more rows than fit, a scrollbar appears on its right
  border. Click or drag it to scroll the list.
- To select text for copying, use your terminal's way around mouse capture,
  if it has one.

## Native picker previews

The preview box shows one of two previews. Press ++ctrl+o++ to switch between
them. The box's title shows `PREVIEW` or `PANE`, with the key in brackets.

- **Command preview**: the output of the preview command, with `{}` replaced
  by the session's directory. The default is
  `eza --icons=always --color=always -la {}`. For a host's session, it runs
  nothing and shows the session's name, host, ID and path.
- **Pane preview**: the live screen of the session's focused terminal in its
  active tab, refreshed every second. It shows the visible screen only, not the
  scrollback. A host's session is read through that host. A directory has no
  screen, so it keeps its command preview.

Switching previews doesn't save the choice. To set the preview that the picker
opens with, set `picker.preview_mode`. With `picker.show_preview = false`, the
picker has no preview and ++ctrl+o++ does nothing. See
[Picker configuration](config/picker.md).

With the preview beside the list, drag the column between them, marked `⋮`,
to resize them. Both keep a minimum width, and the width resets when the picker
closes. A narrow picker stacks the preview below the list. A preview longer than
its box shows a scrollbar on the box's right border.

## Settings controls

Press ++f2++ in the picker, or run `rex-sesh config edit`, to open the settings
editor. Opened from the picker, it returns to the picker when you leave it, and
the picker reloads with any settings you saved.

| Key | Action |
| --- | --- |
| ++up++ / ++down++, ++k++ / ++j++, ++tab++ / ++shift+tab++ | Select a setting. |
| ++enter++ / ++space++ / ++right++ | Toggle a boolean, choose the next choice, or open an editor for text, numbers and lists. |
| ++left++ | Choose the previous choice. On other settings, as ++enter++. |
| ++ctrl+r++ | Revert the selected setting to its loaded value. |
| ++ctrl+s++ | Review all changes before saving. |
| ++esc++ / ++q++ / ++ctrl+c++ | Leave the editor. With unsaved changes, it asks first. |

In the review of your changes:

| Key | Action |
| --- | --- |
| ++y++ | Save the changes. |
| ++n++ / ++esc++ | Return to editing. |
| ++up++ / ++down++, ++k++ / ++j++ | Scroll. |

In a text editor:

| Key | Action |
| --- | --- |
| ++enter++ | Insert a newline. |
| ++ctrl+s++ | Apply the text to the draft. |
| ++esc++ | Cancel the edit. |
| `\t` / `\\` | Write a tab / a backslash. Pasted text is escaped for you. |

In a number editor, ++enter++ or ++ctrl+s++ applies the number, and ++esc++
cancels. The number must be a whole number of at least 1.

In a list editor:

| Key | Action |
| --- | --- |
| ++up++ / ++down++ | Select an item. |
| ++a++ | Add an item. |
| ++enter++ / ++e++ | Edit the selected item. |
| ++d++ / ++delete++ | Delete the selected item. |
| ++ctrl+up++ / ++ctrl+down++ | Move the selected item. |
| ++ctrl+s++ | Apply the list to the draft. |
| ++esc++ | Cancel the list's changes. |

Other screens:

- **Confirm**, before discarding a draft or reloading the file: ++y++
  confirms, ++n++ or ++esc++ returns to editing.
- **Message**, after an error: ++enter++ or ++esc++ returns, and ++up++ /
  ++down++ scroll. When the file changed on disk since the editor read it,
  ++r++ reloads it and discards your draft, after you confirm.
- **Migration**, when the config uses the legacy Sesh-style format: ++y++
  converts it, ++n++, ++esc++ or ++q++ cancels, and ++up++ / ++down++ scroll.
- **Too small**: the editor needs at least 60×18 cells. With less room,
  it keeps your draft and asks you to resize. ++esc++ or ++q++ leaves, and asks
  first when you have unsaved changes.

See [Configuration](config.md) for how the editor saves and converts files.

## fzf picker controls

`rex-sesh picker --fzf`, or `REX_SESH_PICKER=fzf`, uses fzf instead of the
native picker. Its header lists ++enter++ to select, ++ctrl+u++ to clear the
filter, and ++esc++ to cancel. Other keys are fzf's own. The fzf picker
switches after fzf exits.
