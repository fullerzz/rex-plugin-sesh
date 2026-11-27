# Sessions on other hosts

## Sub-features

The Mac picker can list and preview sessions on a connected host. A picker opened inside a host session executes that host's installed binary. SSH destination names and Rex host labels can differ.

## How to get to it (user POV)

On the Mac look for `[host] name`. On a host use the same installed binding or palette action within a host session. Deployment normally uses `just install-host SSH_DEST`.

## Driving it with Rex and just

Use `rex --autostart=false hosts --json` and `rex --autostart=false -S rex-buntu26 ls --json` for the read-only comparison. Match IDs in the Mac [catalog](catalog.md) and check a [host pane preview](previews.md). SSH aliases `buntu26` and `ser8` correspond to `rex-buntu26` and `rex-ser8`. Skip `rex-homelab`.

**Install plan.** From the target checkout, run this in a subshell. The extra executables record and reject an attempted transfer or build; the copied recipe runs in an empty directory. Require a printed plan containing the OS/CPU probe, build, upload and final rename, no call log and no build output.

```sh
(
  set -eu
  work=$(mktemp -d)
  trap 'rm -rf "$work"' EXIT
  cp "$TARGET_REPO/justfile" "$work/justfile"
  mkdir "$work/guards"
  for tool in ssh scp go; do
    printf '%s\n' '#!/bin/sh' 'echo "$0" >> "$GUARD_LOG"' 'exit 99' > "$work/guards/$tool"
    chmod +x "$work/guards/$tool"
  done
  cd "$work"
  PATH="$work/guards:$PATH" GUARD_LOG="$work/calls" just --dry-run install-host buntu26 > "$EVIDENCE/install-plan.txt" 2>&1
  test ! -e "$work/calls"
  test ! -e "$work/bin"
)
```

**Host-local layer.** This optional probe creates a temporary session on the host. Run only when authorized for host probes. Require an existing host install, Remote Control and exclusive ownership of the new probe. Use a fresh `zz-` label and save the creation response:

```sh
HOST=rex-buntu26
probe="zz-sesh-probe-$(date +%s)-$$"
rex --autostart=false -S "$HOST" new --cwd /home/zach --json "$probe" > "$EVIDENCE/host-new.json"
```

Set `SID` to the session ID from that response. Check its label against the host's session list; if creation was ambiguous, inspect the list before retrying. Use the actual bin and launcher literals emitted by this build. The script runs on the Mac and sends the layer request to the host.

```sh
"$TARGET_REPO/bin/rex-sesh" lua > "$EVIDENCE/host-module.lua"
sed -n -e '/^local bin = /p' -e '/^    local start = /p' "$EVIDENCE/host-module.lua" > "$EVIDENCE/host-probe.lua"
cat >> "$EVIDENCE/host-probe.lua" <<'LUA'
local sid = assert(rex.args.sid)
local attached = assert(rex.call("session.attach", {session_id = sid}))
return assert(rex.session.new_layer{
  session_id = sid, window_id = assert(attached.view.active_window_id), focus = false,
  bounds = {0.1, 0.1, 0.8, 0.8},
  layout = rex.layout.block{flavor = "com.superlogical.terminal.shell",
    options = {command = {"/bin/sh", "-c", start, bin, "picker"}}},
})
LUA
rex --autostart=false -S "$HOST" do "$EVIDENCE/host-probe.lua" sid="$SID" --json > "$EVIDENCE/host-layer.json"
B=$(jq -er '.block_ids[0] | select(type == "string" and startswith("block:"))' "$EVIDENCE/host-layer.json")
rex --autostart=false -S "$HOST" capture -b "$B" --trim > "$EVIDENCE/host-frame.txt"
```

Wait for the host rows and ◆ on the probe (move the selection off it if needed). Confirm the Mac path is absent on the target before claiming the host fallback ran. Send Esc to that block with the same `-S`, confirm capture reports that exact block missing, then `rex --autostart=false -S "$HOST" kill "$SID"`. Confirm the recorded ID disappeared from that server's session list. On failure still remove the owned probe by its verified ID.

## Gotchas

`--dry-run` belongs before `install-host`; arguments after the recipe name are SSH destinations. A printed plan does not establish the remote installation version. Never install, update or remove a host binary as part of this check. A missing host or empty host catalog leaves that live case unverified.
