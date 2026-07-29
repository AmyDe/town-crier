#!/usr/bin/env bash
# Watch a PR's required gate and print a one-word verdict, so the orchestrator
# doesn't have to hold a live model turn babysitting `gh pr checks --watch`.
#
# Usage: watch-pr.sh <pr-number>
#   Best run as a background Bash task; re-engage the model on the result.
#
# auto-merge.yml is intentionally disabled in this repo (Claude PR-triage
# routines merge after reviewing CodeRabbit/other comments — see CLAUDE.md,
# "PR merge — no auto-merge, Claude-routine triage"), so a green gate does not
# imply an imminent merge. This script only watches the gate; it never polls
# for or performs a merge itself.
#
# Prints exactly one of:
#   GATE_PASSED      — gate passed; PR is open, ready for triage/merge
#   FAILED: <checks> — one or more checks failed (comma-separated names)
#   TIMEOUT          — checks did not resolve
# Exit code mirrors the verdict (0 passed, 1 failed, 2 timeout).
set -eo pipefail

pr="${1:?usage: watch-pr.sh <pr-number>}"

# Block until every check resolves; --fail-fast exits non-zero on first failure.
if gh pr checks "$pr" --watch --fail-fast >/dev/null 2>&1; then
  echo "GATE_PASSED"; exit 0
fi

# Non-zero: a check failed (or none exist). List the failing check names.
failing=$(gh pr checks "$pr" --json name,bucket \
  -q '.[] | select(.bucket=="fail") | .name' 2>/dev/null | paste -sd, - || true)
if [ -n "$failing" ]; then
  echo "FAILED: $failing"; exit 1
fi
echo "TIMEOUT"; exit 2
