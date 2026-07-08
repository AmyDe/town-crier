#!/usr/bin/env bash
# Generate the user-facing iOS release-notes changelog for TestFlight / App Store.
#
# Why this exists (tc-9sef): the GitHub release body is an *engineering*
# changelog — backend scopes, bead IDs, PR numbers, internal jargon — and must
# never be shown to App Store users verbatim. This script derives concise,
# user-centric notes deterministically, with no LLM or human in the loop, so it
# can run unattended in CD.
#
# Each user-facing change contributes one note; the two sources are MERGED, not
# ranked (an early "first tier wins" design let a single trailer suppress every
# sibling commit's note — that is how a multi-PR range silently dropped the
# onboarding-wizard launch, tc-0557). Per change, in commit order:
#   1. If the commit carries `Release-Note:` trailer(s), use them verbatim.
#      Authored, plain-English copy — the way to get polished notes:
#          Release-Note: Saved searches now refresh the moment an application changes.
#   2. Otherwise, if it is a user-facing mobile/ios/ commit, use its subject with
#      the conventional-commit `type(scope):` prefix, `(#NN)` PR refs and
#      `(tc-xxxx)` bead IDs stripped. Explicitly non-user-facing types
#      (chore/ci/docs/test/build/style/refactor/perf) and version-bump plumbing
#      are excluded. This also catches subjects that LOST their feat/fix prefix
#      during squash-merge (e.g. a plain "iOS onboarding wizard: ..." title).
#   3. If nothing qualifies, a stock catch-all line.
# A commit's own subject is never emitted alongside its trailer (no duplicates).
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

# --- Source 1: authored Release-Note: trailers (verbatim, any commit) --------
# GitHub hard-wraps squash-commit bodies at ~72 chars, so a trailer authored as
# one line can arrive wrapped across several. Join continuation lines until the
# paragraph ends: a blank line, another `Key:` trailer, or a markdown-ish line
# (`---` rule, list bullet, heading, HTML comment). A directly following
# Release-Note: line starts a new note rather than being swallowed.
trailer_notes="$(
  git log --format='%B' "$RANGE" 2>/dev/null \
    | awk '
        function flush() { if (collecting) { print note; collecting = 0 } }
        {
          if (collecting) {
            if ($0 ~ /^[[:space:]]*$/ \
                || $0 ~ /^[[:alnum:]][[:alnum:]-]*:([[:space:]]|$)/ \
                || $0 ~ /^(---|\*|-[[:space:]]|#|<|`)/) {
              flush()
            } else {
              sub(/[[:space:]]+$/, "")
              note = note " " $0
              next
            }
          }
          if ($0 ~ /^[Rr]elease-[Nn]ote:[[:space:]]*/) {
            note = $0
            sub(/^[Rr]elease-[Nn]ote:[[:space:]]*/, "", note)
            sub(/[[:space:]]+$/, "", note)
            collecting = 1
          }
        }
        END { flush() }
      ' || true
)"

# --- Source 2: cleaned mobile/ios subjects for commits WITHOUT a trailer -----
# Skipping trailer-carrying commits is what keeps the two sources from either
# duplicating a change (subject + its own trailer) or — the bug this replaces —
# letting one source suppress the other. Non-user-facing types and version-bump
# plumbing are excluded so they can never leak to App Store users.
subject_notes="$(
  while IFS= read -r h; do
    [ -z "$h" ] && continue
    if git show -s --format='%B' "$h" 2>/dev/null | grep -qiE '^Release-Note:'; then
      continue   # its trailer already covers it (Source 1)
    fi
    git show -s --format='%s' "$h" 2>/dev/null
  done < <(git log --no-merges --format='%H' "$RANGE" -- mobile/ios/ 2>/dev/null) \
    | grep -viE '^(chore|ci|docs|test|build|style|refactor|perf)(\([^)]*\))?!?:' \
    | sed -E 's/^(feat|fix)(\([^)]*\))?!?:[[:space:]]*//' \
    | sed -E 's/[[:space:]]*\(#[0-9]+\)//g; s/[[:space:]]*\(tc-[a-z0-9]+\)//g; s/[[:space:]]+$//' \
    | grep -viE 'bump.*version|marketing version|version bump' || true
)"

# --- Merge, dedupe, tidy -----------------------------------------------------
notes="$(
  printf '%s\n%s\n' "$trailer_notes" "$subject_notes" \
    | awk 'NF && !seen[$0]++ {
        first = substr($0, 1, 1); second = substr($0, 2, 1)
        # Capitalise the lead char, but leave "iOS"/"macOS"-style words alone.
        if (first ~ /[a-z]/ && second !~ /[A-Z]/) $0 = toupper(first) substr($0, 2)
        print
      }'
)"

# --- Stock catch-all ---------------------------------------------------------
if [ -z "$notes" ]; then
  printf '%s\n' "$STOCK"
  exit 0
fi

# Bulletise and cap at TestFlight's limit.
printf '%s\n' "$notes" | sed -E 's/^/- /' | head -c 3900
