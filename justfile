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

# Install rex-sesh on each SSH destination (such as user@hostname, not a Rex
# host label), built for its OS and CPU: at ~/.local/bin/rex-sesh on Linux and
# macOS, or at %LOCALAPPDATA%\Programs\rex-sesh\rex-sesh.exe on Windows
[positional-arguments]
install-host +hosts:
    #!/usr/bin/env bash
    set -euo pipefail
    version=$(git describe --tags --match 'v[0-9]*' --always --dirty 2>/dev/null || printf 'dev')
    ssh_opts=(-o BatchMode=yes -o ConnectTimeout=8)
    # Windows OpenSSH may start cmd.exe or PowerShell; an encoded command
    # reaches PowerShell unchanged through either.
    run_ps() {
        local encoded
        encoded=$(printf '%s' "$2" | iconv -f UTF-8 -t UTF-16LE | base64 | tr -d '\n')
        ssh "${ssh_opts[@]}" "$1" "powershell -NoLogo -NoProfile -NonInteractive -EncodedCommand $encoded" | tr -d '\r'
    }
    ssh_err=$(mktemp)
    trap 'rm -f "$ssh_err"' EXIT
    mkdir -p bin
    for host in "$@"; do
        status=0
        probe=$(ssh "${ssh_opts[@]}" "$host" 'uname -sm' 2>"$ssh_err") || status=$?
        if [ "$status" -eq 255 ]; then
            cat "$ssh_err" >&2
            echo "$host: ssh could not connect. install-host takes an SSH destination, such as user@hostname, not a Rex host label." >&2
            exit 1
        elif [ "$status" -eq 0 ]; then
            read -r os arch <<<"$probe"
        else
            os=Windows
            arch=$(run_ps "$host" '[Console]::Out.Write($env:PROCESSOR_ARCHITECTURE)') || {
                echo "$host: neither uname -sm nor PowerShell reported its OS" >&2
                exit 1
            }
        fi
        case "$os" in
            Linux) goos=linux ;;
            Darwin) goos=darwin ;;
            Windows | MINGW* | MSYS* | CYGWIN*) goos=windows ;;
            *) echo "$host: unsupported OS $os" >&2; exit 1 ;;
        esac
        case "$arch" in
            x86_64 | amd64 | AMD64) goarch=amd64 ;;
            aarch64 | arm64 | ARM64) goarch=arm64 ;;
            *) echo "$host: unsupported CPU $arch" >&2; exit 1 ;;
        esac
        out="bin/rex-sesh-$goos-$goarch"
        if [ "$goos" = windows ]; then
            out="$out.exe"
        fi
        echo "{{ BOLD + BLUE + BG_BLACK }} Building rex-sesh $version for $host ($goos/$goarch)...{{ NORMAL }}"
        CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -ldflags "-X github.com/fullerzz/rex-plugin-sesh/internal/app.Version=$version" -o "$out" ./cmd/rex-sesh
        if [ "$goos" = windows ]; then
            dir=$(run_ps "$host" '$d = Join-Path $env:LOCALAPPDATA "Programs\rex-sesh"; New-Item -ItemType Directory -Force -Path $d | Out-Null; [Console]::Out.Write($d)')
            scp -q "${ssh_opts[@]}" "$out" "$host:${dir//\\//}/rex-sesh.exe.new"
            # Windows can't replace a running .exe, but it can rename one, so
            # a picker that is running keeps its binary.
            run_ps "$host" '$ErrorActionPreference = "Stop"
    $d = Join-Path $env:LOCALAPPDATA "Programs\rex-sesh"
    $exe = Join-Path $d "rex-sesh.exe"
    $old = Join-Path $d "rex-sesh.exe.old"
    Remove-Item -LiteralPath $old -Force -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $exe) { Move-Item -LiteralPath $exe -Destination $old -Force }
    Move-Item -LiteralPath (Join-Path $d "rex-sesh.exe.new") -Destination $exe
    & $exe --version
    exit $LASTEXITCODE'
            echo "Installed rex-sesh $version on $host at $dir\\rex-sesh.exe"
            continue
        fi
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
    @echo 'Unit canceled/op assume=exact'
    @echo 'Unit completed/op assume=exact'
    go test -run '^$' -bench=. -benchmem -count={{count}} ./internal/picker

# Compare two saved benchmark runs
bench-compare base candidate:
    go tool benchstat "base={{base}}" "candidate={{candidate}}"

# Exercise the release recipe, workflows, changelog range and tag resolution in throwaway repositories
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

    # The range keeps the Herdr history from before the fork out of the changelog.
    range=$(bash .github/scripts/changelog-range.sh)
    if [[ -n "$version" ]]; then
        mise exec -- git-cliff --tag "$version" "$range"
    else
        mise exec -- git-cliff "$range"
    fi

# Validate, tag, and trigger the GitHub release workflow
[confirm("Create and push release " + tag + "?")]
release $tag:
    #!/usr/bin/env bash
    set -euo pipefail

    # The tag is the only version source: vMAJOR.MINOR.PATCH, optionally with a
    # -PRERELEASE suffix. release.yml and changelog.yml check the same version_re.
    version_re='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'
    if [[ ! "$tag" =~ $version_re ]]; then
        echo "Release tag must be vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-PRERELEASE, got: $tag" >&2
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
    remote_tag=$(git ls-remote --tags origin "refs/tags/$tag")
    if [[ -n "$remote_tag" ]]; then
        echo "Tag already exists on origin: $tag" >&2
        exit 1
    fi
    if [[ -z "${GITHUB_TOKEN:-}" ]]; then
        echo "GITHUB_TOKEN is required for GitHub changelog metadata" >&2
        exit 1
    fi

    just check
    just build
    ./bin/rex-sesh --version
    # With no Rex server, list prints a warning and still exits 0.
    ./bin/rex-sesh list --json --config testdata/rex-sesh.toml >/dev/null
    # The range keeps the Herdr history from before the fork out of the changelog.
    range=$(bash .github/scripts/changelog-range.sh)
    mise exec -- git-cliff --tag "$tag" --output CHANGELOG.md "$range"
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

# Run all checks for code changes
check: lint fmt-check test test-release-ref
    @echo "{{ BOLD + GREEN + BG_BLACK }} All checks passed!{{ NORMAL }}"
