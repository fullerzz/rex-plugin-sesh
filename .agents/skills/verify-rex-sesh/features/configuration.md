# Lua action and launcher

## Sub-features

The generator embeds the binary's path, registers the picker action and optionally binds a key. Its shell launcher also handles a host with no installed binary.

## How to get to it (user POV)

Use `rex-sesh lua` or `rex-sesh lua --key 'ctrl+a>s'`. Users normally load the module from Rex's init.lua. Verification can load the generated file directly.

## Driving it with the CLI

From the checkout, generate both variants into the private evidence directory:

```sh
./bin/rex-sesh lua > "$EVIDENCE/plain.lua"
./bin/rex-sesh lua --key 'ctrl+a>s' > "$EVIDENCE/keyed.lua"
rex --autostart=false config check "$EVIDENCE/plain.lua"
rex --autostart=false config check "$EVIDENCE/keyed.lua"
rex --autostart=false config check
go test -v -count=1 ./internal/app -run '^TestPickerStart'
```

Require one action in both generated files, zero bindings in the first and one in the second. Inspect the embedded binary path and `sesh_picker` action. The last config check reads the user's existing config. The focused Go tests execute the actual launcher with disposable paths, covering the local arguments, host fallback without a Mac client ID, and exit 127 with “Sesh cannot start” and installation guidance when both lookups fail.

## Gotchas

The config checker loads Lua locally. The installed server and keymap can differ. Never overwrite or reload the user's config during verification. Missing-binary presentation in a real layer (including its Exited badge) is a separate observation from shell stderr.
