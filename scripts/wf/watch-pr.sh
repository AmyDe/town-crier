#!/usr/bin/env bash
# Watch a PR's required gate and print a one-word verdict, so the orchestrator
# doesn't have to hold a live model turn babysitting `gh pr checks --watch`.
#
# Usage: watch-pr.sh <pr-number>
#   Best run as a background Bash task; re-engage the model on the result.
#
# auto-merge.yml is intentionally disabled in this repo (Claude PR-triage
# routines merge after reviewing review comments — see CLAUDE.md,
# "PR merge — no auto-merge, Claude-routine triage"), so a green gate does not
# imply an imminent merge. This script only watches the gate; it never polls
# for or performs a merge itself.
#
# Prints exactly one of:
#   GATE_PASSED      — gate passed; PR is open, ready for triage/merge
#   FAILED: <checks> — a check failed or was cancelled, or the PR Gate did not
#                      pass (comma-separated "name (bucket)")
#   TIMEOUT          — checks did not resolve
# Exit code mirrors the verdict (0 passed, 1 failed, 2 timeout).
set -eo pipefail

pr="${1:?usage: watch-pr.sh <pr-number>}"
gate="PR Gate"

# Block until every check resolves; --fail-fast exits non-zero on first failure.
# The exit code alone is not a verdict: --watch also exits 0 when checks end
# cancelled or skipped (for example in a GitHub Actions outage).
gh pr checks "$pr" --watch --fail-fast >/dev/null 2>&1 || true

checks=$(gh pr checks "$pr" --json name,bucket 2>/dev/null || true)
gate_bucket=$(jq -r --arg gate "$gate" '.[] | select(.name==$gate) | .bucket' <<<"${checks:-[]}" | head -n1)
if [ "$gate_bucket" = "pass" ]; then
  echo "GATE_PASSED"; exit 0
fi

failing=$(jq -r '.[] | select(.bucket=="fail" or .bucket=="cancel") | "\(.name) (\(.bucket))"' <<<"${checks:-[]}" | paste -sd, - || true)
if [ -n "$failing" ]; then
  echo "FAILED: $failing"; exit 1
fi
if [ -n "$gate_bucket" ] && [ "$gate_bucket" != "pending" ]; then
  echo "FAILED: $gate ($gate_bucket)"; exit 1
fi
echo "TIMEOUT"; exit 2
