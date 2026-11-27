---
name: verify-rex-sesh
description: Verify the rex-sesh CLI, native Rex picker, Lua action and host launcher after catalog, integration, preview or keybinding changes.
disable-model-invocation: true
---

# Verify rex-sesh

Read the [feature map](features/README.md) and choose the entry points affected by the change. The fixture journey runs anywhere the project builds. The interactive journey needs a Rex terminal, an installed action and Remote Control already enabled. Leave that setting and the user's Rex config alone.

## Launch

Set `TARGET_REPO` to the absolute checkout under test. Set `SKILL_FILE` to the absolute path of the SKILL.md actually loaded by your host, including when discovery followed a symlink. In a POSIX-style shell:

```sh
VERIFY_DIR=$(dirname "$(realpath "$SKILL_FILE")")
cd "$TARGET_REPO" && just build
EVIDENCE="$TARGET_REPO/scratch/verify-rex-sesh-$(date +%Y%m%d-%H%M%S)-$$"
```

Use the pinned build tools from mise, plus zoxide for the fixture. The CLI needs no long-running service. For repository-wide verification run `just check` and `just build-docs` from the target and retain both exit codes.

## Doctor

Run `"$TARGET_REPO/bin/rex-sesh" --version`. For the live journey run `rex --autostart=false whoami --json` and confirm the session and block match the caller's `REX_SESSION` and `REX_BLOCK`. Check `rex actions` for `sesh_picker`; use `rex keymap` to identify the installed binding. If Remote Control is unavailable or the action is missing, report the live entry point blocked. Keep one driver per live tab.

## Drive

```sh
(cd /tmp && uv run --no-project python "$VERIFY_DIR/scripts/verify.py" "$TARGET_REPO" "$EVIDENCE")
```

The helper launches with empty private state, validates a fixture config and exercises both catalog commands. Its `passed` file means the row, configuration and cleanup checks succeeded. For rendered rows, keys and cancellation, follow [the live picker journey](features/picker.md). The installed action may reference a different binary than the checkout. Inspect its generated module before attributing installed-action results to a build.

**Refuse Enter, clicking a row, or any `session.select` on a real session ID unless the user explicitly requests a live switch.** This includes the current row. A directory selection can create a session and update zoxide. Have the user confirm the visible destination for authorized switches. Rex has no API for which session the app is showing.

## Evidence

Each isolated invocation uses a new evidence directory. Keep its command log, version, revision, binary hash, stdout, stderr and exit files. Record live action arguments, returned IDs, before/after frames and the final lookup of the closed block alongside them. Report each entry point as passed, failed, blocked or untested. Keep live terminal content and paths private. Avoid collecting `rex client ls --json` principal fields.

## Cleanup

The helper removes its temporary runtime on success or exception. Confirm the evidence survived with `test -f "$EVIDENCE/passed"` and inspect the directory for leftover `runtime-*` after interruption. Live drives have separate teardown: send Esc only to the recorded picker block, confirm it disappeared, and clean up only sessions created by your own authorized probe. Keep proof files after teardown.

## Helpers

`scripts/verify.py TARGET_REPO NEW_EVIDENCE_DIR` is executable through uv and uses Python's standard library. Independent fixture runs have separate HOME, Rex and zoxide state. The fake Rex supplies catalog responses at the same executable boundary used by the app tests. Live server discovery, installed host binaries, UI rendering and app switches require their own evidence.

Use `/maintain-verification-skill` when the feature map or commands change.
