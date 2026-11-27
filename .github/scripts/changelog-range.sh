#!/usr/bin/env bash
# Print the git-cliff commit range for the rex-sesh changelog.
#
# rex-sesh is a fork of herdr-plugin-sesh and keeps its Git history, but that
# Herdr history must never appear in the changelog. The Herdr release tags were
# deleted, so without a range git-cliff would put every Herdr commit into the
# first Rex release. fork_base is the last Herdr commit. The fork starts at its
# child e5d8aeb ("refactor!: strip the Herdr integration from the Rex fork").
#
# Every git-cliff run (the justfile, release.yml and changelog.yml) passes the
# range this script prints, so fork_base is held here only. Don't use
# git-cliff's --current or --latest: they ignore the range, and with a single
# release tag they include all Herdr history.
#
# Usage:
#   changelog-range.sh        fork_base..HEAD, for the whole changelog
#   changelog-range.sh TAG    the commits that TAG releases, for its release
#                             notes: from the previous version tag, or from
#                             fork_base when there is none, to TAG

set -euo pipefail

fork_base=8a937e231880bbf2dfaabac7482dff103999f4f8

if ! git rev-parse --verify --quiet "${fork_base}^{commit}" >/dev/null; then
  echo "Changelog fork base $fork_base is missing; fetch the full history" >&2
  exit 1
fi

tag=${1:-}
if [[ -z "$tag" ]]; then
  end=HEAD
else
  end="refs/tags/$tag"
  if ! git rev-parse --verify --quiet "${end}^{commit}" >/dev/null; then
    echo "Release tag is missing: $end" >&2
    exit 1
  fi
fi
if ! git merge-base --is-ancestor "$fork_base" "$end"; then
  echo "Changelog fork base $fork_base is not an ancestor of $end" >&2
  exit 1
fi

start=$fork_base
# A version tag at or before fork_base is Herdr history, so the range never
# starts there.
if [[ -n "$tag" ]] &&
  previous=$(git describe --tags --abbrev=0 --match 'v[0-9]*' "${end}^" 2>/dev/null) &&
  ! git merge-base --is-ancestor "refs/tags/$previous" "$fork_base"; then
  start="refs/tags/$previous"
fi

printf '%s..%s\n' "$start" "$end"
