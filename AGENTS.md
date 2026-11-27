# Repository Guidelines

## Project Structure & Module Organization

This repository is `rex-sesh`, a Go CLI that brings a Sesh-style session picker to the Rex terminal. It is forked from herdr-plugin-sesh. The Herdr client, plugin manifest, and history watcher have been removed. The docs site and the release tooling still describe Herdr until they are rewritten.

- `cmd/rex-sesh/` contains the executable entry point.
- `internal/app/` owns CLI routing and command behavior.
- `internal/config/`, `internal/model/`, `internal/sources/`, `internal/state/`, and related packages hold domain logic.
- `docs/` contains the Markdown and assets for the Zensical documentation site.
- `zensical.toml` defines the documentation navigation and theme; `pyproject.toml` and `uv.lock` pin its Python toolchain.
- `testdata/` stores fixture TOML used by tests and smoke checks.
- Rex integration is a Lua config module: `rex-sesh lua` prints it from `internal/app/sesh.lua.tmpl`, and users load it from `init.lua`. There is no plugin manifest.
- `bin/` and `site/` are generated build output; do not treat them as source.

## Build, Test, and Development Commands

- `mise install` installs pinned tools from `mise.toml`: Go, `cargo-binstall`, `golangci-lint`, `gotestsum`, `just`, `fzf`, `bat`, `eza`, `git-cliff`, `prek`, and `uv`.
- `just` lists available development recipes from the `justfile`.
- `just build-docs` runs the same strict Zensical build used for GitHub Pages; `just serve-docs` previews the site locally.
- `just fmt-check` checks formatting; `just fmt` applies it.
- `just lint` runs `golangci-lint run ./...`.
- `just test` runs the race-enabled test suite through `gotestsum`.
- `just build` builds the local binary at `bin/rex-sesh`.
- `just run` runs `./cmd/rex-sesh` through `go run`.
- `./bin/rex-sesh --version` smoke-tests the built CLI.
- `./bin/rex-sesh list --json --config testdata/sesh.toml` checks fixture-backed session listing.

CI should run formatting, vet, tests, build, and CLI smoke checks; mirror those checks before opening a pull request. Documentation changes should also pass `just build-docs`. Prefer `just` recipes so local checks use pinned tools.

## Coding Style & Naming Conventions

Use idiomatic Go formatting and short, lowercase package names. Keep command orchestration in `internal/app` and reusable logic in narrow `internal/*` packages. Prefer table tests only when they reduce repetition. Use `xh` instead of `curl` and `uv` when Python is needed.

Tool versions belong in `mise.toml`; update that file when adding or changing shared developer tooling.

`internal/picker/` renders the native picker with bloom (`bloomfx.dev/bloom`). Before changing it, read https://bloomfx.dev/llms.txt and fetch only the linked `.md` pages you need (`xh --ignore-stdin`); do not load `llms-full.txt` whole. bloom is pre-1.0 and its site documents the latest release. When the site and code disagree, trust `go doc` for the version pinned in `go.mod`.

## Testing Guidelines

Tests use Go's standard `testing` package and live beside the code as `*_test.go`. Name tests as `Test<Behavior>` and keep file fixtures in `testdata/`. New behavior should include the smallest focused regression test.

Use Testify `require` for prerequisites (errors, lengths, and nil checks before accessing results) and `assert` for independent expectations. Keep `require` on the test goroutine; report worker errors back to it. `testifylint` checks assertion usage through `just lint`.

## Commit & Pull Request Guidelines

Recent commits use Conventional Commit-style subjects, for example `feat: cache session list when enabled` and `fix: pad native picker top border`. Keep subjects imperative and scoped when useful (`feat:`, `fix:`, `ci:`, `docs:`).

Pull requests should include a short description, linked issue when applicable, and the exact validation commands run. Include CLI output or screenshots only when changing user-visible command behavior.

## Release & Configuration Notes

Release tags must start with `v`. The release version source is undecided. `just release`, `release.yml`, `changelog.yml`, and `cliff.toml` still read the removed `herdr-plugin.toml`, so fix them before the first release. Configuration lookup order is documented in `README.md`; preserve compatibility with Sesh-style TOML and existing `testdata/sesh.toml` fixtures.
