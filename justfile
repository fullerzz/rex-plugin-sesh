_default:
    @just --list

run:
    go run ./cmd/rex-sesh

# Explore the settings TUI using a disposable copy, or pass a native config path
settings-demo $config='':
    #!/usr/bin/env bash
    set -euo pipefail
    target="$config"
    if [[ -z "$target" ]]; then
        target=$(mktemp "${TMPDIR:-/tmp}/rex-sesh-settings.XXXXXX")
        trap 'rm -f "$target" "$target.settings.lock"' EXIT
        cp testdata/rex-sesh.toml "$target"
    fi
    go run ./cmd/rex-sesh config edit --config "$target"

# Build the documentation site
build-docs:
    uv run --frozen zensical build --clean --strict

# Build latest and all release-tag documentation for GitHub Pages
build-docs-versions:
    uv run --frozen python .github/scripts/build-docs-versions.py

# Exercise versioned documentation with isolated Git history
test-docs-versions:
    uv run --frozen python .github/scripts/test-docs-versions.py

# Preview the documentation site locally
serve-docs:
    uv run --frozen zensical serve

# Clean build artifacts
clean:
    @echo "{{ BOLD + RED + BG_BLACK }}󰿞 Cleaning build artifacts...{{ NORMAL }}"
    rm -f ./bin/rex-sesh ./bin/rex-sesh-*

# Build the binary with Go's greenteagc garbage collector enabled
build:
    @echo "{{ BOLD + BLUE + BG_BLACK }} Building the project...{{ NORMAL }}"
    mkdir -p bin
    go build -ldflags "-X github.com/fullerzz/rex-plugin-sesh/internal/app.Version=$(git describe --tags --match 'v[0-9]*' --always --dirty 2>/dev/null || printf 'dev')" -o bin/rex-sesh ./cmd/rex-sesh

# Install rex-sesh at ~/.local/bin/rex-sesh on each SSH host, built for its OS and CPU
[positional-arguments]
install-host +hosts:
    #!/usr/bin/env bash
    set -euo pipefail
    version=$(git describe --tags --match 'v[0-9]*' --always --dirty 2>/dev/null || printf 'dev')
    ssh_opts=(-o BatchMode=yes -o ConnectTimeout=8)
    mkdir -p bin
    for host in "$@"; do
        read -r os arch < <(ssh "${ssh_opts[@]}" "$host" 'uname -sm')
        case "$os" in
            Linux) goos=linux ;;
            Darwin) goos=darwin ;;
            *) echo "$host: unsupported OS $os" >&2; exit 1 ;;
        esac
        case "$arch" in
            x86_64 | amd64) goarch=amd64 ;;
            aarch64 | arm64) goarch=arm64 ;;
            *) echo "$host: unsupported CPU $arch" >&2; exit 1 ;;
        esac
        out="bin/rex-sesh-$goos-$goarch"
        echo "{{ BOLD + BLUE + BG_BLACK }} Building rex-sesh $version for $host ($goos/$goarch)...{{ NORMAL }}"
        CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -ldflags "-X github.com/fullerzz/rex-plugin-sesh/internal/app.Version=$version" -o "$out" ./cmd/rex-sesh
        # Upload beside the target, then rename it into place, so a picker
        # that is running keeps its binary.
        ssh "${ssh_opts[@]}" "$host" 'mkdir -p "$HOME/.local/bin"'
        scp -q "${ssh_opts[@]}" "$out" "$host:.local/bin/.rex-sesh.new"
        ssh "${ssh_opts[@]}" "$host" 'chmod 755 "$HOME/.local/bin/.rex-sesh.new" && mv -f "$HOME/.local/bin/.rex-sesh.new" "$HOME/.local/bin/rex-sesh" && "$HOME/.local/bin/rex-sesh" --version'
        echo "Installed rex-sesh $version on $host at ~/.local/bin/rex-sesh"
    done

# Run linters on the codebase
lint:
    @echo "{{ BOLD + GREEN + BG_BLACK }} Running linters...{{ NORMAL }}"
    mise exec -- golangci-lint run ./...

# Format the codebase
fmt:
    @echo "{{ BOLD + MAGENTA + BG_BLACK }} Formatting the code...{{ NORMAL }}"
    mise exec -- golangci-lint fmt ./...

# Check formatting without rewriting files
fmt-check:
    @echo "{{ BOLD + MAGENTA + BG_BLACK }} Checking formatting...{{ NORMAL }}"
    mise exec -- golangci-lint fmt --diff ./...

# Run tests
test:
    @echo "{{ BOLD + BLUE + BG_BLACK }} Running tests...{{ NORMAL }}"
    gotestsum --format-icons=octicons --format=pkgname -- -race ./...

# Run the application benchmark suite
bench count='1':
    @echo 'Unit commands/op better=lower assume=exact'
    @echo 'Unit canceled/op assume=exact'
    @echo 'Unit completed/op assume=exact'
    go test -run '^$' -bench=. -benchmem -count={{count}} ./internal/sources ./internal/picker

# Compare two saved benchmark runs
bench-compare base candidate:
    go tool benchstat "base={{base}}" "candidate={{candidate}}"

# Exercise release tag resolution against a same-named branch/tag collision
test-release-ref:
    bash .github/scripts/test-release-ref.sh

# Preview the changelog on stdout, optionally using the next release version
preview-changelog $version='':
    #!/usr/bin/env bash
    set -euo pipefail

    if [[ -z "${GITHUB_TOKEN:-}" ]]; then
        echo "GITHUB_TOKEN is required for GitHub changelog metadata" >&2
        exit 1
    fi

    if [[ -n "$version" ]]; then
        mise exec -- git-cliff --tag "$version"
    else
        mise exec -- git-cliff
    fi

# Validate, tag, and trigger the GitHub release workflow
[confirm("Create and push release " + tag + "?")]
release $tag:
    #!/usr/bin/env bash
    set -euo pipefail

    expected_tag="v$(sed -nE 's/^version = "([^"]+)".*/\1/p' herdr-plugin.toml)"
    if [[ "$tag" != "$expected_tag" ]]; then
        echo "Tag mismatch: $tag != manifest $expected_tag" >&2
        exit 1
    fi
    current_branch=$(git branch --show-current)
    if [[ "$current_branch" != "main" ]]; then
        echo "Releases must be created from main, got: ${current_branch:-detached HEAD}" >&2
        exit 1
    fi
    if [[ -n "$(git status --porcelain)" ]]; then
        echo "Working tree must be clean before releasing" >&2
        exit 1
    fi
    if git rev-parse --verify --quiet "refs/tags/$tag" >/dev/null; then
        echo "Tag already exists: $tag" >&2
        exit 1
    fi
    if [[ -z "${GITHUB_TOKEN:-}" ]]; then
        echo "GITHUB_TOKEN is required for GitHub changelog metadata" >&2
        exit 1
    fi

    just check
    just build
    ./bin/rex-sesh --version
    ./bin/rex-sesh list --json --config testdata/rex-sesh.toml >/dev/null
    mise exec -- git-cliff --tag "$tag" --output CHANGELOG.md
    if ! grep -Fq "## ${tag} " CHANGELOG.md; then
        echo "Generated changelog is missing $tag" >&2
        exit 1
    fi
    if git diff --quiet -- CHANGELOG.md; then
        echo "Changelog is already up to date for $tag" >&2
        exit 1
    fi
    git add CHANGELOG.md
    git tag -a "$tag" -m "Release $tag"
    if ! git commit -m "docs(CHANGELOG): update CHANGELOG.md [skip ci]"; then
        git tag -d "$tag"
        git restore --staged --worktree -- CHANGELOG.md
        exit 1
    fi
    git push --atomic origin HEAD "refs/tags/$tag"

# Run all checks for code changes. test-release-ref returns once release
# tooling reads a version source other than the removed herdr-plugin.toml.
check: lint fmt-check test
    @echo "{{ BOLD + GREEN + BG_BLACK }} All checks passed!{{ NORMAL }}"
