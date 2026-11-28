#!/usr/bin/env bash

set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
workflow="$repo_root/.github/workflows/release.yml"
changelog_workflow="$repo_root/.github/workflows/changelog.yml"
helper="$repo_root/.github/scripts/checkout-release-ref.sh"
range_script="$repo_root/.github/scripts/changelog-range.sh"
justfile="$repo_root/justfile"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# shellcheck disable=SC2016 # Match the workflow's literal shell variables.
if ! grep -Fq 'bash .github/scripts/checkout-release-ref.sh "$RELEASE_REF"' "$workflow"; then
  echo 'release workflow must resolve the explicit tag ref through checkout-release-ref.sh' >&2
  exit 1
fi
# shellcheck disable=SC2016 # Match the workflow's literal shell variables.
if ! grep -Fq 'if [ "$remote_commit" != "$EXPECTED_COMMIT" ]; then' "$workflow"; then
  echo 'publish must reject a tag that moved after build verification' >&2
  exit 1
fi
# shellcheck disable=SC2016 # Match the workflow's literal GitHub expression.
if ! grep -Fq 'GH_REPO: ${{ github.repository }}' "$workflow"; then
  echo 'publish must provide repository context to gh release commands' >&2
  exit 1
fi

# The tag is the only version source. Every release entry point must accept
# the same tag form: vMAJOR.MINOR.PATCH with an optional -PRERELEASE suffix.
version_re_of() {
  sed -nE "s/^[[:space:]]*version_re='([^']*)'\$/\\1/p" "$1"
}
version_re=$(version_re_of "$justfile")
if [ -z "$version_re" ] ||
  [ "$(version_re_of "$workflow")" != "$version_re" ] ||
  [ "$(version_re_of "$changelog_workflow")" != "$version_re" ]; then
  echo 'the release recipe, release.yml and changelog.yml must check tags with the same version_re' >&2
  exit 1
fi
# shellcheck disable=SC2016 # Match the files' literal shell variables.
for file in "$justfile" "$workflow"; do
  if ! grep -Fq 'if [[ ! "$tag" =~ $version_re ]]; then' "$file"; then
    echo "$file must reject tags that don't match version_re" >&2
    exit 1
  fi
done
# shellcheck disable=SC2016 # Match the workflow's literal shell variables.
if ! grep -Fq 'if [[ ! "$TAG" =~ $version_re ]]; then' "$changelog_workflow"; then
  echo "changelog workflow must reject tags that don't match version_re" >&2
  exit 1
fi
for good_tag in v0.1.0 v1.2.3 v10.20.30 v1.2.3-rc.1 v1.2.3-alpha-2 v0.1.0-dryrun; do
  if [[ ! "$good_tag" =~ $version_re ]]; then
    echo "version_re must accept $good_tag" >&2
    exit 1
  fi
done
for bad_tag in 0.1.0 v1 v1.2 v1.2.3.4 v01.2.3 v1.02.3 v1.2.3- v1.2.3-rc..1 v1.2.3+build \
  'v1.2.3-rc/1' 'v1.2.3-rc#1' 'v#probe' 'v%2Fprobe' 'v/path' 'v?probe'; do
  if [[ "$bad_tag" =~ $version_re ]]; then
    echo "version_re must reject $bad_tag" >&2
    exit 1
  fi
done

# shellcheck disable=SC2016 # Match the workflow's literal shell variables.
if ! grep -Fq 'name="rex-sesh_${version}_${goos}_${goarch}"' "$workflow" ||
  ! grep -Fq 'out="dist/work/${name}/bin/rex-sesh"' "$workflow"; then
  echo 'release archives must be named rex-sesh_VERSION_GOOS_GOARCH and hold bin/rex-sesh' >&2
  exit 1
fi
if ! grep -Fxq '            windows/amd64' "$workflow"; then
  echo 'release workflow must build windows/amd64' >&2
  exit 1
fi
# shellcheck disable=SC2016 # Match the workflow's literal shell variables.
if ! grep -Fq '(cd dist/work && zip -qr "../release/${name}.zip" "$name")' "$workflow" ||
  ! grep -Fq 'sha256sum -- *.tar.gz *.zip > checksums.txt' "$workflow"; then
  echo 'the Windows release must be a zip listed in checksums.txt' >&2
  exit 1
fi
# shellcheck disable=SC2016 # Match the workflow's literal shell variables.
if ! grep -Fq 'cp README.md LICENSE "dist/work/${name}/"' "$workflow"; then
  echo 'release archives must include README.md and LICENSE' >&2
  exit 1
fi
# shellcheck disable=SC2016 # Match the workflow's literal shell variables.
if ! grep -Fq 'test "$("$binary" --version)" = "rex-sesh ${version}"' "$workflow"; then
  echo 'release workflow must verify the packaged binary version' >&2
  exit 1
fi
if ! grep -Fq -- '--config testdata/rex-sesh.toml' "$workflow"; then
  echo 'release workflow must smoke-test list with the rex-sesh fixture' >&2
  exit 1
fi
# shellcheck disable=SC2016 # Match the workflow's literal shell variables.
if ! grep -Fq "echo \"gh release download \${tag} --repo \${GITHUB_REPOSITORY} --pattern '\${archive}' --pattern checksums.txt\"" "$workflow"; then
  echo 'generated install instructions must download the release tag' >&2
  exit 1
fi
if ! grep -Fxq "          ~/.local/bin/rex-sesh lua --key 'ctrl+a>s' > ~/.config/rex/sesh.lua" "$workflow"; then
  echo 'generated install instructions must set up Rex with rex-sesh lua from the installed binary' >&2
  exit 1
fi
if grep -Eqi 'herdr plugin install|herdr-plugin\.toml' "$workflow" "$changelog_workflow" "$justfile"; then
  echo 'release tooling must not use the Herdr plugin manifest or installer' >&2
  exit 1
fi

# The version reaches the binary only through -ldflags -X, so the workflow's
# module path must match go.mod.
module=$(sed -nE "s/^[[:space:]]*module='([^']*)'\$/\\1/p" "$workflow")
go_module=$(sed -nE 's/^module[[:space:]]+([^[:space:]]+)$/\1/p' "$repo_root/go.mod")
if [ -z "$go_module" ] || [ "$module" != "$go_module/internal/app" ]; then
  echo "release workflow must inject the version into $go_module/internal/app, got: ${module:-<none>}" >&2
  exit 1
fi
(cd "$repo_root" && CGO_ENABLED=0 go build -ldflags "-X ${module}.Version=1.2.3" -o "$tmp/rex-sesh" ./cmd/rex-sesh)
if [ "$("$tmp/rex-sesh" --version)" != 'rex-sesh 1.2.3' ]; then
  echo 'release build flags must inject the tag version into the binary' >&2
  exit 1
fi

# Every git-cliff run passes the range from changelog-range.sh, which keeps the
# Herdr history from before the fork out of the changelog.
# shellcheck disable=SC2016 # Match the workflows' literal shell variables.
if ! grep -Fq 'range=$(bash .github/scripts/changelog-range.sh "$TAG")' "$workflow" ||
  ! grep -Fq 'args: --strip header --verbose ${{ steps.changelog-range.outputs.range }}' "$workflow"; then
  echo 'release notes must use the changelog range for the release tag' >&2
  exit 1
fi
# shellcheck disable=SC2016 # Match the workflows' literal shell variables.
if ! grep -Fq 'range=$(bash .github/scripts/changelog-range.sh)' "$changelog_workflow" ||
  ! grep -Fq 'args: --verbose ${{ steps.changelog-range.outputs.range }}' "$changelog_workflow"; then
  echo 'changelog workflow must use the changelog range' >&2
  exit 1
fi
if grep -Eq '^[[:space:]]*args:.*--(current|latest|unreleased)' "$workflow" "$changelog_workflow"; then
  echo 'git-cliff --current, --latest and --unreleased ignore the changelog range' >&2
  exit 1
fi
if ! grep -Fq '  pull-requests: read' "$changelog_workflow"; then
  echo 'changelog workflow must allow git-cliff to read pull request metadata' >&2
  exit 1
fi

git -C "$tmp" init -q
git -C "$tmp" config user.email test@example.com
git -C "$tmp" config user.name 'Release workflow test'

printf 'tag\n' >"$tmp/version"
git -C "$tmp" add version
git -C "$tmp" commit -qm 'tag commit'
tag_commit=$(git -C "$tmp" rev-parse HEAD)
git -C "$tmp" tag -am 'annotated release tag' v1.2.3
git -C "$tmp" tag v1.2.4

git -C "$tmp" switch -qc v1.2.3
printf 'branch\n' >"$tmp/version"
git -C "$tmp" commit -qam 'same-named branch commit'
branch_commit=$(git -C "$tmp" rev-parse HEAD)

resolved=$(cd "$tmp" && bash "$helper" refs/tags/v1.2.3)
head=$(git -C "$tmp" rev-parse HEAD)

if [ "$resolved" != "$tag_commit" ] || [ "$head" != "$tag_commit" ]; then
  echo "expected explicit tag commit $tag_commit, resolved $resolved with HEAD $head" >&2
  exit 1
fi
if [ "$resolved" = "$branch_commit" ]; then
  echo "resolved same-named branch commit $branch_commit instead of tag" >&2
  exit 1
fi

lightweight=$(cd "$tmp" && bash "$helper" refs/tags/v1.2.4)
if [ "$lightweight" != "$tag_commit" ]; then
  echo "expected lightweight tag commit $tag_commit, resolved $lightweight" >&2
  exit 1
fi

if missing=$(cd "$tmp" && bash "$helper" refs/tags/v9.9.9 2>&1); then
  echo 'expected a missing release tag to fail' >&2
  exit 1
fi
case "$missing" in
  *'Requested release tag is missing: refs/tags/v9.9.9'*) ;;
  *) echo "missing release tag failed unclearly: $missing" >&2; exit 1 ;;
esac

for unsafe_tag in 'v#probe' 'v%2Fprobe' 'v/path' 'v?probe'; do
  if unsafe=$(cd "$tmp" && bash "$helper" "refs/tags/$unsafe_tag" 2>&1); then
    echo "expected unsafe release tag $unsafe_tag to fail" >&2
    exit 1
  fi
  case "$unsafe" in
    *"Release tag must match project-safe syntax: $unsafe_tag"*) ;;
    *) echo "unsafe release tag $unsafe_tag failed unclearly: $unsafe" >&2; exit 1 ;;
  esac
done

# This repository lacks the real fork base, so the range must fail loudly
# instead of letting git-cliff read the whole history.
if missing_base=$(cd "$tmp" && bash "$range_script" 2>&1); then
  echo 'changelog range must fail when the fork base is missing' >&2
  exit 1
fi
case "$missing_base" in
  *'Changelog fork base '*' is missing; fetch the full history'*) ;;
  *) echo "missing changelog fork base failed unclearly: $missing_base" >&2; exit 1 ;;
esac

just_bin=$(mise which just)
git_cliff_bin=$(mise which git-cliff)

"$git_cliff_bin" --from-context "$repo_root/.github/scripts/testdata/changelog-context.json" \
  --output "$tmp/rendered-changelog.md"
if grep -Eq '.## v[0-9]' "$tmp/rendered-changelog.md"; then
  echo 'first-time contributor entries must preserve the next release heading newline' >&2
  exit 1
fi
if ! grep -Fxq '* @direct-contributor made their first contribution' "$tmp/rendered-changelog.md"; then
  echo 'first-time contributor entries without a pull request must omit the PR suffix' >&2
  exit 1
fi
if ! grep -Fq 'by @orhun in [#389](https://github.com/fullerzz/rex-plugin-sesh/pull/389)' \
  "$tmp/rendered-changelog.md"; then
  echo 'commit pull request metadata must link to the pull request' >&2
  exit 1
fi
if ! grep -Fq '@first-timer made their first contribution in [#79](https://github.com/fullerzz/rex-plugin-sesh/pull/79)' \
  "$tmp/rendered-changelog.md"; then
  echo 'first-time contributor metadata must link to the pull request' >&2
  exit 1
fi

release_tools="$tmp/release-tools"
mkdir -p "$release_tools"
printf '%s\n' '#!/usr/bin/env bash' 'exit 0' >"$release_tools/just"
# shellcheck disable=SC2016 # Write literal shell variables into the fake mise script.
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  'if [ "$1" != exec ] || [ "$2" != -- ] || [ "$3" != git-cliff ]; then' \
  '  echo "unexpected mise invocation: $*" >&2' \
  '  exit 1' \
  'fi' \
  'shift 3' \
  'exec "$RELEASE_TEST_GIT_CLIFF" "$@"' \
  >"$release_tools/mise"
chmod +x "$release_tools/just" "$release_tools/mise"

if preview_error=$(
  cd "$repo_root"
  env -u GITHUB_TOKEN \
    GIT_CLIFF_OFFLINE=true \
    PATH="$release_tools:$PATH" \
    RELEASE_TEST_GIT_CLIFF="$git_cliff_bin" \
    "$just_bin" preview-changelog 2>&1
); then
  echo 'changelog preview must require an authenticated GitHub metadata request' >&2
  exit 1
fi
case "$preview_error" in
  *'GITHUB_TOKEN is required for GitHub changelog metadata'*) ;;
  *) echo "missing changelog preview token failed unclearly: $preview_error" >&2; exit 1 ;;
esac

# Each release repository starts with Herdr history: a leftover Herdr release
# tag, then the last Herdr commit, which becomes the fork base in its copy of
# changelog-range.sh. None of it may reach the changelog.
setup_release_repo() {
  release_repo=$1
  release_remote=$2
  git init -q -b main "$release_repo"
  git init -q --bare "$release_remote"
  git -C "$release_repo" config user.email test@example.com
  git -C "$release_repo" config user.name 'Release recipe test'
  git -C "$release_repo" config commit.gpgsign false
  git -C "$release_repo" config tag.gpgsign false
  printf 'herdr\n' >"$release_repo/herdr.txt"
  git -C "$release_repo" add herdr.txt
  git -C "$release_repo" commit -qm 'feat: herdr-only history'
  git -C "$release_repo" tag v0.9.0
  printf 'last herdr commit\n' >>"$release_repo/herdr.txt"
  git -C "$release_repo" commit -qam 'fix: last herdr commit'
  release_base=$(git -C "$release_repo" rev-parse HEAD)

  cp "$justfile" "$repo_root/cliff.toml" "$release_repo/"
  mkdir -p "$release_repo/bin" "$release_repo/.github/scripts"
  sed "s/^fork_base=.*/fork_base=$release_base/" "$range_script" \
    >"$release_repo/.github/scripts/changelog-range.sh"
  if ! grep -Fxq "fork_base=$release_base" "$release_repo/.github/scripts/changelog-range.sh"; then
    echo 'changelog-range.sh must hold the fork base in a fork_base= line' >&2
    exit 1
  fi
  printf '## Unreleased\n' >"$release_repo/CHANGELOG.md"
  printf '%s\n' '#!/usr/bin/env bash' 'exit 0' >"$release_repo/bin/rex-sesh"
  chmod +x "$release_repo/bin/rex-sesh"
  git -C "$release_repo" add .
  git -C "$release_repo" commit -qm 'feat: rex fork feature'
  git -C "$release_repo" remote add origin "$release_remote"
}

run_release_recipe() {
  release_repo=$1
  release_tag=${2:-v1.2.3}
  (
    cd "$release_repo"
    GITHUB_TOKEN=test-token \
      GIT_CLIFF_OFFLINE=true \
      PATH="$release_tools:$PATH" \
      RELEASE_TEST_GIT_CLIFF="$git_cliff_bin" \
      "$just_bin" --yes release "$release_tag"
  )
}

generate_changelog() {
  changelog_repo=$1
  shift
  (cd "$changelog_repo" && GIT_CLIFF_OFFLINE=true "$git_cliff_bin" "$@" 2>/dev/null)
}

assert_no_herdr_history() {
  if grep -Eiq 'herdr|v0\.9\.0' "$1"; then
    echo "$2 must leave out the Herdr history before the fork base:" >&2
    cat "$1" >&2
    exit 1
  fi
}

missing_token_repo="$tmp/release-missing-token"
missing_token_remote="$tmp/release-missing-token.git"
setup_release_repo "$missing_token_repo" "$missing_token_remote"
if missing_token_error=$(
  cd "$missing_token_repo"
  env -u GITHUB_TOKEN \
    GIT_CLIFF_OFFLINE=true \
    PATH="$release_tools:$PATH" \
    RELEASE_TEST_GIT_CLIFF="$git_cliff_bin" \
    "$just_bin" --yes release v1.2.3 2>&1
); then
  echo 'release must require an authenticated GitHub metadata request' >&2
  exit 1
fi
case "$missing_token_error" in
  *'GITHUB_TOKEN is required for GitHub changelog metadata'*) ;;
  *) echo "missing release token failed unclearly: $missing_token_error" >&2; exit 1 ;;
esac

for bad_tag in v1.2 1.2.3 v1.2.3+build; do
  if bad_tag_error=$(run_release_recipe "$missing_token_repo" "$bad_tag" 2>&1); then
    echo "release recipe must reject tag $bad_tag" >&2
    exit 1
  fi
  case "$bad_tag_error" in
    *"Release tag must be vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-PRERELEASE, got: $bad_tag"*) ;;
    *) echo "invalid release tag $bad_tag failed unclearly: $bad_tag_error" >&2; exit 1 ;;
  esac
done

git -C "$missing_token_repo" push -q origin HEAD:refs/tags/v1.2.3
if remote_tag_error=$(run_release_recipe "$missing_token_repo" 2>&1); then
  echo 'release recipe must reject a tag that already exists on origin' >&2
  exit 1
fi
case "$remote_tag_error" in
  *'Tag already exists on origin: v1.2.3'*) ;;
  *) echo "existing origin tag failed unclearly: $remote_tag_error" >&2; exit 1 ;;
esac
if git -C "$missing_token_repo" rev-parse --verify --quiet refs/tags/v1.2.3 >/dev/null; then
  echo 'rejected release must not create a local tag' >&2
  exit 1
fi

success_repo="$tmp/release-success"
success_remote="$tmp/release-success.git"
setup_release_repo "$success_repo" "$success_remote"
success_base=$release_base
run_release_recipe "$success_repo"
success_head=$(git -C "$success_repo" rev-parse HEAD)
success_parent=$(git -C "$success_repo" rev-parse HEAD^)
success_tag=$(git -C "$success_repo" rev-parse 'refs/tags/v1.2.3^{commit}')
remote_head=$(git --git-dir="$success_remote" rev-parse refs/heads/main)
remote_tag=$(git --git-dir="$success_remote" rev-parse 'refs/tags/v1.2.3^{commit}')
if [ "$success_tag" != "$success_parent" ] ||
  [ "$remote_head" != "$success_head" ] ||
  [ "$remote_tag" != "$success_tag" ]; then
  echo 'release recipe must atomically push the changelog commit and its tagged parent' >&2
  exit 1
fi
if [ "$(git -C "$success_repo" diff-tree --no-commit-id --name-only -r HEAD)" != CHANGELOG.md ]; then
  echo 'release recipe must commit only CHANGELOG.md' >&2
  exit 1
fi
if ! grep -Fq '## v1.2.3 ' "$success_repo/CHANGELOG.md" ||
  ! grep -Fq 'Rex fork feature' "$success_repo/CHANGELOG.md"; then
  echo 'release changelog must list the fork commits under the release tag' >&2
  exit 1
fi
assert_no_herdr_history "$success_repo/CHANGELOG.md" 'release changelog'

notes_range=$(cd "$success_repo" && bash .github/scripts/changelog-range.sh v1.2.3)
if [ "$notes_range" != "${success_base}..refs/tags/v1.2.3" ]; then
  echo "first release notes must start at the fork base, not at a Herdr tag: got $notes_range" >&2
  exit 1
fi
generate_changelog "$success_repo" --strip header "$notes_range" >"$tmp/notes-v1.2.3.md"
if ! grep -Fq '## v1.2.3 ' "$tmp/notes-v1.2.3.md" ||
  ! grep -Fq 'Rex fork feature' "$tmp/notes-v1.2.3.md"; then
  echo 'first release notes must list the fork commits' >&2
  exit 1
fi
assert_no_herdr_history "$tmp/notes-v1.2.3.md" 'first release notes'

# The changelog workflow regenerates the changelog from the tag checkout; with
# the same metadata that must match what the release recipe committed to main.
git -C "$success_repo" checkout -q --detach refs/tags/v1.2.3
full_range=$(cd "$success_repo" && bash .github/scripts/changelog-range.sh)
generate_changelog "$success_repo" --output "$tmp/post-tag-CHANGELOG.md" "$full_range"
git -C "$success_repo" show main:CHANGELOG.md >"$tmp/main-CHANGELOG.md"
if ! cmp -s "$tmp/main-CHANGELOG.md" "$tmp/post-tag-CHANGELOG.md"; then
  echo 'tag-triggered changelog generation must be a no-op on main' >&2
  exit 1
fi

# Exercise the actual workflow step from a dirty tag checkout while main already
# contains the release recipe's changelog commit.
awk '
  /- name: Prepare main branch update/ { step = 1; next }
  step && /run: \|/ { script = 1; next }
  script && /^      - name:/ { exit }
  script { sub(/^          /, ""); print }
' "$changelog_workflow" >"$tmp/prepare-changelog.sh"
cp "$tmp/post-tag-CHANGELOG.md" "$success_repo/CHANGELOG.md"
printf '\nUpdated GitHub metadata\n' >>"$success_repo/CHANGELOG.md"
cp "$success_repo/CHANGELOG.md" "$tmp/expected-changelog.md"
mkdir "$tmp/changelog-runner"
(
  cd "$success_repo"
  RUNNER_TEMP="$tmp/changelog-runner" bash "$tmp/prepare-changelog.sh"
)
if [ "$(git -C "$success_repo" branch --show-current)" != main ] ||
  [ "$(git -C "$success_repo" rev-parse HEAD)" != "$success_head" ] ||
  ! cmp -s "$success_repo/CHANGELOG.md" "$tmp/expected-changelog.md"; then
  echo 'changelog workflow must preserve generated content when switching from the tag to main' >&2
  exit 1
fi

# A second release gets its own section, and its notes start at the first tag.
git -C "$success_repo" restore -- CHANGELOG.md
printf 'second\n' >"$success_repo/second.txt"
git -C "$success_repo" add second.txt
git -C "$success_repo" commit -qm 'feat: second rex feature'
run_release_recipe "$success_repo" v1.2.4
if ! grep -Fq '## v1.2.4 ' "$success_repo/CHANGELOG.md" ||
  ! grep -Fq '## v1.2.3 ' "$success_repo/CHANGELOG.md" ||
  ! grep -Fq 'Second rex feature' "$success_repo/CHANGELOG.md"; then
  echo 'second release changelog must keep both release sections' >&2
  exit 1
fi
assert_no_herdr_history "$success_repo/CHANGELOG.md" 'second release changelog'
notes_range=$(cd "$success_repo" && bash .github/scripts/changelog-range.sh v1.2.4)
if [ "$notes_range" != 'refs/tags/v1.2.3..refs/tags/v1.2.4' ]; then
  echo "second release notes must start at the previous release tag: got $notes_range" >&2
  exit 1
fi
generate_changelog "$success_repo" --strip header "$notes_range" >"$tmp/notes-v1.2.4.md"
if ! grep -Fq '## v1.2.4 ' "$tmp/notes-v1.2.4.md" ||
  ! grep -Fq 'Second rex feature' "$tmp/notes-v1.2.4.md" ||
  grep -Fq 'Rex fork feature' "$tmp/notes-v1.2.4.md"; then
  echo 'second release notes must list only the commits since the previous release' >&2
  exit 1
fi

if existing_tag_error=$(run_release_recipe "$success_repo" v1.2.4 2>&1); then
  echo 'release recipe must reject an existing tag' >&2
  exit 1
fi
case "$existing_tag_error" in
  *'Tag already exists: v1.2.4'*) ;;
  *) echo "existing release tag failed unclearly: $existing_tag_error" >&2; exit 1 ;;
esac

branch_repo="$tmp/release-branch"
branch_remote="$tmp/release-branch.git"
setup_release_repo "$branch_repo" "$branch_remote"
git -C "$branch_repo" switch -qc release/v1.2.3
if branch_error=$(run_release_recipe "$branch_repo" 2>&1); then
  echo 'release recipe must reject non-main branches' >&2
  exit 1
fi
case "$branch_error" in
  *'Releases must be created from main, got: release/v1.2.3'*) ;;
  *) echo "non-main release failed unclearly: $branch_error" >&2; exit 1 ;;
esac
if git -C "$branch_repo" rev-parse --verify --quiet refs/tags/v1.2.3 >/dev/null; then
  echo 'rejected non-main release must not create a tag' >&2
  exit 1
fi

failure_repo="$tmp/release-commit-failure"
failure_remote="$tmp/release-commit-failure.git"
setup_release_repo "$failure_repo" "$failure_remote"
printf '%s\n' '#!/usr/bin/env bash' 'exit 1' >"$failure_repo/.git/hooks/pre-commit"
chmod +x "$failure_repo/.git/hooks/pre-commit"
if run_release_recipe "$failure_repo" >/dev/null 2>&1; then
  echo 'release recipe must propagate changelog commit failures' >&2
  exit 1
fi
if git -C "$failure_repo" rev-parse --verify --quiet refs/tags/v1.2.3 >/dev/null; then
  echo 'failed changelog commit must remove the unpushed release tag' >&2
  exit 1
fi
if ! git -C "$failure_repo" diff --quiet -- CHANGELOG.md ||
  ! git -C "$failure_repo" diff --cached --quiet -- CHANGELOG.md; then
  echo 'failed changelog commit must restore CHANGELOG.md in the worktree and index' >&2
  exit 1
fi
printf '%s\n' '#!/usr/bin/env bash' 'exit 0' >"$failure_repo/.git/hooks/pre-commit"
if ! run_release_recipe "$failure_repo" >/dev/null 2>&1; then
  echo 'release recipe must be retryable after a transient commit failure' >&2
  exit 1
fi
