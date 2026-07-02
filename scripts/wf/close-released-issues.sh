#!/usr/bin/env bash
# After a release, close GitHub issues whose linked beads are all closed.
#
# Usage: close-released-issues.sh <version> <release-url>
#
# The `bead-created` label marks issues converted to beads; the linkage lives in
# a triage comment of the form `… bead **tc-xxxx** …`. An issue is closed only
# when every linked bead is closed. Best-effort: a failure here never fails the
# release. Prints a one-line tally. Used by the `release` skill (Step 7).
set -eo pipefail

version="${1:?usage: close-released-issues.sh <version> <release-url>}"
release_url="${2:?need release url}"

issues=$(gh issue list --state=open --label=bead-created --limit=200 \
  --json number -q '.[].number' 2>/dev/null || true)
if [ -z "$issues" ]; then
  echo "No GH issues ready to close."
  exit 0
fi

closed=()
for n in $issues; do
  beads=$(gh issue view "$n" --json comments \
    --jq '[.comments[].body] | join(" ")' 2>/dev/null \
    | grep -oE 'tc-[a-z0-9]+' | sort -u || true)
  [ -z "$beads" ] && continue

  all_closed=1
  for b in $beads; do
    st=$(bd show "$b" --json 2>/dev/null | jq -r '.[0].status' 2>/dev/null || echo "")
    if [ "$st" != "closed" ]; then
      all_closed=0
      break
    fi
  done

  if [ "$all_closed" = "1" ]; then
    if gh issue close "$n" \
      --comment "All linked beads are closed; shipped in [${version}](${release_url})." \
      >/dev/null 2>&1; then
      closed+=("#$n")
    fi
  fi
done

if [ "${#closed[@]}" -gt 0 ]; then
  echo "Closed ${#closed[@]} GH issue(s) whose beads all shipped: ${closed[*]}"
else
  echo "No GH issues ready to close."
fi
