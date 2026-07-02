#!/usr/bin/env bash
# Generate categorized release notes from conventional commits.
#
# Usage: release-notes.sh [from-ref]
#   from-ref defaults to the latest v* tag; notes cover from-ref..HEAD.
#
# Prints the markdown skeleton to stdout in the project's established format.
# The mechanical 80% (bump, grouping, PR links) is done here; the caller adds
# the optional dash-explanations where a subject alone isn't clear. Used by the
# `release` skill, whose manual categorization rules remain the fallback.
set -eo pipefail

from="${1:-}"
if [ -z "$from" ]; then
  from=$(git tag --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1 || true)
fi
if [ -n "$from" ]; then
  commits=$(git log "${from}..HEAD" --no-merges --format='%s')
else
  commits=$(git log --no-merges --format='%s')
fi

feat=(); fix=(); perf=(); refactor=(); testing=(); cicd=(); docs=(); chore=(); other=()

while IFS= read -r line; do
  [ -z "$line" ] && continue
  if [[ "$line" =~ ^([a-z]+)(\([^\)]*\))?!?:\ .*$ ]]; then
    type="${BASH_REMATCH[1]}"
    case "$type" in
      feat)     feat+=("- **${line}**") ;;
      fix)      fix+=("- **${line}**") ;;
      perf)     perf+=("- **${line}**") ;;
      refactor) refactor+=("- **${line}**") ;;
      test)     testing+=("- **${line}**") ;;
      ci)       cicd+=("- ${line}") ;;
      docs)     docs+=("- **${line}**") ;;
      chore)
        if [[ "$line" =~ ^chore\(ci\) ]]; then cicd+=("- ${line}"); else chore+=("- ${line}"); fi ;;
      *)        other+=("- ${line}") ;;
    esac
  else
    other+=("- ${line}")
  fi
done <<< "$commits"

emit() {
  local heading="$1"; shift
  [ "$#" -eq 0 ] && return 0
  printf '\n### %s\n' "$heading"
  printf '%s\n' "$@"
}

echo "## What's Changed"
emit "Features"      "${feat[@]}"
emit "Bug Fixes"     "${fix[@]}"
emit "Performance"   "${perf[@]}"
emit "Refactoring"   "${refactor[@]}"
emit "Testing"       "${testing[@]}"
emit "CI/CD"         "${cicd[@]}"
emit "Documentation" "${docs[@]}"
emit "Chores"        "${chore[@]}"
emit "Other"         "${other[@]}"
