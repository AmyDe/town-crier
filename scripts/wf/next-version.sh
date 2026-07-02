#!/usr/bin/env bash
# Compute the next semver release tag.
#
# Usage: next-version.sh [patch|minor|major]
#   No argument → auto-detect from conventional commits since the latest tag:
#     any `type!:` subject or `BREAKING CHANGE` in a body → major
#     otherwise any `feat` → minor
#     otherwise → patch
#
# Prints the next version (e.g. v0.15.63) to stdout. Read-only; touches no tags.
# Used by the `release` skill; the skill's manual rules remain the fallback.
set -eo pipefail

git fetch --tags --quiet origin 2>/dev/null || true

latest=$(git tag --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1 || true)
[ -z "$latest" ] && latest="v0.0.0"

level="${1:-auto}"
if [ "$level" = "auto" ]; then
  if [ "$latest" = "v0.0.0" ]; then
    log=$(git log --format='%s%n%b')
  else
    log=$(git log "${latest}..HEAD" --format='%s%n%b')
  fi
  if printf '%s\n' "$log" | grep -qE '^[a-z]+(\([^)]*\))?!:' \
     || printf '%s\n' "$log" | grep -q 'BREAKING CHANGE'; then
    level=major
  elif printf '%s\n' "$log" | grep -qE '^feat(\([^)]*\))?:'; then
    level=minor
  else
    level=patch
  fi
fi

ver="${latest#v}"
IFS=. read -r maj min pat <<<"$ver"
case "$level" in
  major) maj=$((maj + 1)); min=0; pat=0 ;;
  minor) min=$((min + 1)); pat=0 ;;
  patch) pat=$((pat + 1)) ;;
  *) echo "unknown bump level: $level" >&2; exit 2 ;;
esac

echo "v${maj}.${min}.${pat}"
