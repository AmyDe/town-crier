#!/usr/bin/env bash
# Single source of truth for detect-changes/action.yml's category-prefix and
# ignore-allowlist data (issue #835 slices 1+2). Sourced (not executed) by
# both:
#   - the "Detect changed paths" step (per-changed-file categorization,
#     fail-closed catch-all), and
#   - the "Coverage assertion" step (whole-repo-tree structural check).
#
# Edit ONLY this file when a new top-level directory, or a new category, is
# introduced — both consuming steps read from here, so there is nothing else
# to update.
#
# shellcheck disable=SC2034  # consumed by scripts that `source` this file
#
# CATEGORY_PREFIXES: detect-changes output category name -> the top-level
# path prefix whose changes flip that category to true. mobile/ios and
# mobile/android are two categories nested one level under the mobile/
# top-level directory; mobile/ itself is a container, not a category — the
# coverage-assertion step handles that by descending into mobile/ one level
# rather than expecting it as a bare prefix match.
declare -A CATEGORY_PREFIXES=(
  [go]="api-go"
  [cli]="cli"
  [ios]="mobile/ios"
  [android]="mobile/android"
  [web]="web"
  [infra]="infra"
)

# IGNORE_ALLOWLIST_*: top-level paths that legitimately need no pr-gate lane
# (docs, repo tooling, repo metadata) — changes here don't flip any category
# and don't trip the fail-closed catch-all in "Detect changed paths".
IGNORE_ALLOWLIST_DIRS=(docs .beads .claude scripts)
IGNORE_ALLOWLIST_FILES=(LICENSE .gitignore .gitattributes)
IGNORE_ALLOWLIST_GLOBS=("*.md")
