#!/usr/bin/env bash
# Watch a PR's required gate and print a one-word verdict, so the orchestrator
# doesn't have to hold a live model turn babysitting `gh pr checks --watch`.
#
# Usage: watch-pr.sh <pr-number>
#   Best run as a background Bash task; re-engage the model on the result.
#
# Prints exactly one of:
#   MERGED           — gate passed and the PR was squash-merged by auto-merge.yml
#   MERGED_PENDING   — gate passed; merge not yet observed (auto-merge will finish)
#   FAILED: <checks> — one or more checks failed (comma-separated names)
#   TIMEOUT          — checks did not resolve
# Exit code mirrors the verdict (0 merged/pending, 1 failed, 2 timeout).
set -eo pipefail

pr="${1:?usage: watch-pr.sh <pr-number>}"

# Block until every check resolves; --fail-fast exits non-zero on first failure.
if gh pr checks "$pr" --watch --fail-fast >/dev/null 2>&1; then
  # Gate green — auto-merge.yml squash-merges shortly. Poll briefly for MERGED.
  i=0
  while [ "$i" -lt 30 ]; do
    state=$(gh pr view "$pr" --json state -q '.state' 2>/dev/null || echo "")
    if [ "$state" = "MERGED" ]; then
      echo "MERGED"; exit 0
    fi
    i=$((i + 1))
    sleep 5
  done
  echo "MERGED_PENDING"; exit 0
fi

# Non-zero: a check failed (or none exist). List the failing check names.
failing=$(gh pr checks "$pr" --json name,bucket \
  -q '.[] | select(.bucket=="fail") | .name' 2>/dev/null | paste -sd, - || true)
if [ -n "$failing" ]; then
  echo "FAILED: $failing"; exit 1
fi
echo "TIMEOUT"; exit 2
