# Native picker

## Sub-features

The native picker renders sessions, marks the caller's session with ◆, shows agent status and moves the selection in response to keys.

## How to get to it (user POV)

Run `rex-sesh picker`, invoke **Sesh: Pick Session** in the palette, or use the installed binding (commonly `ctrl+a s`). The CLI action below exercises the installed layer with focus disabled. Palette and binding need separate observations. The optional `picker --fzf` has its own interface.

## Driving it with Rex

After doctor and the fixture run, open over your own tab. Record commands, and issue each next key only after inspecting its preceding frame.

```sh
rex --autostart=false do -s "$REX_SESSION" -b "$REX_BLOCK" sesh_picker focus=false --json > "$EVIDENCE/open.json"
B=$(jq -er '.block_ids[0] | select(type == "string" and startswith("block:"))' "$EVIDENCE/open.json")
rex --autostart=false capture -b "$B" --trim > "$EVIDENCE/initial.txt"
rex --autostart=false send-key -b "$B" ctrl+j
rex --autostart=false capture -b "$B" --trim > "$EVIDENCE/moved.txt"
```

Wait for the session list and footer before moving. Allow time for redraw and recapture as needed. Compare the rail before and after movement. Move off the calling session to see its ◆. Compare status with the agent actually running there. Working animates, done uses ●, and an idle session need not show activity. If the fixture lacks the relevant agent state, record the gap. Run the [preview check](previews.md), then Esc, including on an assertion failure.

```sh
rex --autostart=false send-key -b "$B" esc
rex --autostart=false capture -b "$B" --trim
```

The final capture must fail with a missing target for that exact block. A connection error leaves cleanup unverified.

## Gotchas

The selection rail can cover ◆. A focused palette or binding launch can move keyboard focus. Reserve it for a user-driven check. Apply the live-switch restriction in [SKILL.md](../SKILL.md#drive) to every entry point.
