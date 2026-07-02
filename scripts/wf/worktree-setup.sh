#!/usr/bin/env bash
# Create a bd-managed worktree ready for a worker, applying the known bd
# workarounds so the orchestrator doesn't narrate the five-step recipe by hand.
#
# Usage: worktree-setup.sh <name> [--branch <branch>]
#
# Does, in order:
#   1. fetch origin and reset local main to origin/main (bd bases off HEAD)
#   2. `bd worktree create` (GH#3311-safe; never `git worktree add`)
#   3. symlink .beads/dolt-server.port into the worktree (GH#3421)
#   4. chmod 700 the worktree's .beads (gastownhall/beads#3593)
#   5. reset the worktree branch to origin/main and verify
#
# Prints the worktree path as the final line. Remove steps 3-4 when the upstream
# bd fixes ship. Anchored to the repo root, so a drifted CWD can't misfire it.
set -eo pipefail

name="${1:?usage: worktree-setup.sh <name> [--branch <branch>]}"
shift || true
branch=""
if [ "${1:-}" = "--branch" ]; then
  branch="${2:?--branch needs a value}"
fi

root=$(git rev-parse --show-toplevel)

git -C "$root" fetch origin --quiet
git -C "$root" branch -f main origin/main 2>/dev/null || true

if [ -n "$branch" ]; then
  (cd "$root" && bd worktree create "$name" --branch "$branch" >/dev/null)
else
  (cd "$root" && bd worktree create "$name" >/dev/null)
fi

# Resolve the worktree path from git (handles both <repo>/<name> and nested layouts).
wt=$(git -C "$root" worktree list --porcelain \
  | awk -v n="/$name" '/^worktree /{p=$2} substr(p, length(p)-length(n)+1)==n{print p; exit}')
[ -z "$wt" ] && wt="$root/$name"

# GH#3421: dolt-server.port is not propagated into the worktree.
if [ -f "$root/.beads/dolt-server.port" ] && [ ! -e "$wt/.beads/dolt-server.port" ]; then
  mkdir -p "$wt/.beads"
  rel=$(python3 -c 'import os,sys; print(os.path.relpath(sys.argv[1], sys.argv[2]))' \
    "$root/.beads/dolt-server.port" "$wt/.beads")
  ln -sf "$rel" "$wt/.beads/dolt-server.port"
fi

# gastownhall/beads#3593: tighten .beads perms so bd stops warning.
[ -d "$wt/.beads" ] && chmod 700 "$wt/.beads"

# bd bases the new branch off the orchestrator tree's current HEAD, not origin/main.
git -C "$wt" reset --hard origin/main --quiet

echo "$wt"
