# Session and directory catalog

## Sub-features

The default picker groups local sessions, host sessions in Rex host order, and ranked zoxide directories. A configured-only workspace appears in the CLI list.

## How to get to it (user POV)

Run `rex-sesh list --json` or `rex-sesh picker --json`. Open the native picker to see the display labels.

## Driving it with the CLI

Run the helper in [Drive](../SKILL.md#drive). Inspect `list.out` and `picker.out`. The two Rex rows carry session IDs, the current one is attached, and the host row has machine `fixture-host`. Two paths ending in `docs` retain their distinguishing parents. The directory named `api` yields to the open session even though their paths differ. The helper checks the config bytes and registered directories after the drive.

For the installed environment capture `"$TARGET_REPO/bin/rex-sesh" picker --json` and stderr separately. Match session IDs with `rex ls --json` on the corresponding server. Under default order, require each host group before the directories. Compare the names with the private live picker frame, where host rows display as `[host] name`.

## Gotchas

Personal source and sort settings can change the order. An unavailable host may yield a warning and partial results. A missing zoxide executable or inaccessible database can yield zero directories even with exit 0. Live row counts vary. JSON does not exercise selection or rendering.
