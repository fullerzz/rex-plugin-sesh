# Command and pane previews

## Sub-features

Local rows show a directory or session summary. Host rows show remote session metadata. Pane mode reads the visible screen of the selected session.

## How to get to it (user POV)

Open the native picker, highlight a row and use the configured preview toggle (default Ctrl+O).

## Driving it with Rex

From the [open picker](picker.md), select a row with Ctrl+J or Ctrl+K and capture its name and PREVIEW contents. For a host row check the host, session ID and path against its JSON row. Toggle and capture:

```sh
rex --autostart=false send-key -b "$B" ctrl+o
rex --autostart=false capture -b "$B" --trim > "$EVIDENCE/toggled.txt"
```

Wait for PANE and a refresh. Compare a recognizable line with a read-only capture of that row's JSON `block_id` on its server (`rex --autostart=false -S "$HOST" capture -b "$ROW_BLOCK" --trim` for a host). Toggle back and confirm PREVIEW returns. Highlight a directory in pane mode and confirm it keeps its directory preview. Finish the picker cleanup.

## Gotchas

A hidden preview or a remapped key affects this check. Narrow layouts can stack the panes. Screen contents can change each second. Record only private evidence suitable for the selected pane.
