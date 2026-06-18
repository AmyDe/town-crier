#!/usr/bin/env bash
# Generate the user-facing iOS release-notes changelog for TestFlight / App Store.
#
# Why this exists (tc-9sef): the GitHub release body is an *engineering*
# changelog — backend scopes, bead IDs, PR numbers, internal jargon — and must
# never be shown to App Store users verbatim. This script derives concise,
# user-centric notes deterministically, with no LLM or human in the loop, so it
# can run unattended in CD.
#
# Resolution order (first tier that yields any line wins):
#   1. `Release-Note:` commit trailers in the range. Authored, plain-English,
#      shipped verbatim. This is the way to get polished copy: when you make a
#      user-facing iOS change, add a trailer, e.g.
#          Release-Note: Saved searches now refresh the moment an application changes.
#   2. mobile/ios/ feat & fix commit subjects, with the conventional-commit
#      `type(scope):` prefix, `(#NN)` PR refs and `(tc-xxxx)` bead IDs stripped.
#      No backend, no infra, no chores, no internal IDs.
#   3. A stock catch-all line.
#
# Usage: ios-release-notes.sh <previous-tag> <current-ref>
#   <previous-tag> may be empty (first release / workflow_dispatch) — the range
#   then becomes the full history reachable from <current-ref>.
#
# Prints the changelog to stdout. Output is capped at 3900 chars (TestFlight's
# changelog limit is 4000). Runs on macOS CI, so it sticks to BSD-portable
# sed/awk/grep — no GNU-only constructs (e.g. sed \U).

set -euo pipefail

PREV="${1:-}"
CURRENT="${2:-HEAD}"

if [ -n "$PREV" ]; then
  RANGE="${PREV}..${CURRENT}"
else
  RANGE="$CURRENT"
fi

STOCK="Bug fixes and performance improvements."

# --- Tier 1: Release-Note: trailers (authored, verbatim) ---------------------
# `|| true` guards against grep exiting 1 (no match) tripping pipefail.
notes="$(
  git log --format='%B' "$RANGE" 2>/dev/null \
    | grep -iE '^Release-Note:' \
    | sed -E 's/^[Rr]elease-[Nn]ote:[[:space:]]*//' \
    | awk 'NF && !seen[$0]++' || true
)"

# --- Tier 2: mobile/ios feat/fix subjects (cleaned) --------------------------
if [ -z "$notes" ]; then
  notes="$(
    git log --no-merges --format='%s' "$RANGE" -- mobile/ios/ 2>/dev/null \
      | grep -E '^(feat|fix)(\([^)]*\))?!?:' \
      | sed -E 's/^(feat|fix)(\([^)]*\))?!?:[[:space:]]*//' \
      | sed -E 's/[[:space:]]*\(#[0-9]+\)//g; s/[[:space:]]*\(tc-[a-z0-9]+\)//g; s/[[:space:]]+$//' \
      | awk 'NF && !seen[$0]++ { print toupper(substr($0,1,1)) substr($0,2) }' || true
  )"
fi

# --- Tier 3: stock catch-all -------------------------------------------------
if [ -z "$notes" ]; then
  printf '%s\n' "$STOCK"
  exit 0
fi

# Bulletise and cap at TestFlight's limit.
printf '%s\n' "$notes" | sed -E 's/^/- /' | head -c 3900
