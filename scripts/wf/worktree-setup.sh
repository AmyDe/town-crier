#!/usr/bin/env bash
# Create a bd-managed worktree ready for a worker, applying the known bd
# workarounds so the orchestrator doesn't narrate the five-step recipe by hand.
#
# Usage: worktree-setup.sh <name> [--branch <branch>]
#
# Worktrees are created at <repo>/.claude/worktrees/<name>, NOT at <repo>/<name>.
# EnterWorktree auto-approves paths under .claude/worktrees/ and raises a
# permission prompt (not answerable from settings) for anything else, so the
# location is load-bearing, not cosmetic. bd takes a path in place of a bare
# name, and still resolves `bd worktree remove <name>` by basename. The blanket
# `.claude/worktrees/` line in .gitignore also stops bd appending a per-worktree
# ignore line for every tree it creates.
#
# Does, in order:
#   1. fetch origin and reset local main to origin/main (bd bases off HEAD)
#   2. `bd worktree create .claude/worktrees/<name>` (GH#3311-safe; never `git worktree add`)
#   3. symlink .beads/dolt-server.port into the worktree (GH#3421)
#   4. chmod 700 the worktree's .beads (gastownhall/beads#3593)
#   5. reset the worktree branch to origin/main and verify
#
# Prints the worktree path as the final line. Remove steps 3-4 when the upstream
# bd fixes ship. Anchored to the repo root, so a drifted CWD can't misfire it.
#
# bd's SEC-003 safety check rejects a BEADS_DIR under /home/* or /Users/* that
# falls outside the current user's home. A normal local checkout is always under
# the real home, so this never fires there.
#
# The HOME override below NO LONGER DEFEATS IT and is kept only as a harmless
# no-op for older bd versions. As of 1.1.2 (internal/beads/context.go,
# isPathInSafeBoundary) bd resolves the home directory from the account database
# via user.Current(), which the source comments say is deliberately "not
# affected by $HOME manipulation". So in a sandbox running as root (passwd home
# /root) with the repo under /home/<user>/..., bd worktree create fails with
# "BEADS_DIR points to unsafe location" whatever HOME is set to — verified under
# both HOME=/root and HOME=/home/user (tc-aj2pn). Don't re-test it.
#
# That is why cloud sessions skip worktrees entirely: .claude/require-worktree.sh
# exits early when CLAUDE_CODE_REMOTE=true, since an ephemeral single-session
# container already provides the isolation worktrees give locally. This script
# is therefore a local-only path in practice.
set -eo pipefail

name="${1:?usage: worktree-setup.sh <name> [--branch <branch>]}"
shift || true
branch=""
if [ "${1:-}" = "--branch" ]; then
  branch="${2:?--branch needs a value}"
fi

# Accept a bare name only. A caller passing a path would land the tree outside
# .claude/worktrees/ and reintroduce the permission prompt.
case "$name" in
  */*|.*) echo "worktree-setup.sh: <name> must be a bare name, got '$name'" >&2; exit 2 ;;
esac

# Resolve the MAIN repo root, not the current tree: run from inside a worktree,
# --show-toplevel returns that worktree and would nest the new tree inside it.
# --git-common-dir always points at the main repo's .git, from any worktree.
root=$(dirname "$(git rev-parse --path-format=absolute --git-common-dir)")
rel=".claude/worktrees/$name"
wt="$root/$rel"

git -C "$root" fetch origin --quiet
# Silence stdout too: `git branch -f` prints "branch 'main' set up to track ..."
# on stdout, which would corrupt `wt=$(worktree-setup.sh <name>)`.
git -C "$root" branch -f main origin/main >/dev/null 2>&1 || true

bd_home="$HOME"
case "$root" in
  "$HOME"/*|"$HOME") ;;                             # already safe — leave $HOME alone
  /home/*|/Users/*) bd_home=$(dirname "$root") ;;   # HOME/checkout mismatch — fall back
esac

if [ -n "$branch" ]; then
  (cd "$root" && HOME="$bd_home" bd worktree create "$rel" --branch "$branch" >/dev/null)
else
  (cd "$root" && HOME="$bd_home" bd worktree create "$rel" >/dev/null)
fi

# Confirm git registered it where we asked — EnterWorktree requires both the
# .claude/worktrees/ location and a `git worktree list` entry. Matched in pure
# bash: `grep` is proxied through RTK in this repo and mangles piped output.
found=0
while IFS= read -r line; do
  if [ "$line" = "worktree $wt" ]; then found=1; break; fi
done < <(git -C "$root" worktree list --porcelain)
if [ "$found" -ne 1 ]; then
  echo "worktree-setup.sh: expected worktree at $wt, not found in git worktree list" >&2
  exit 1
fi

# GH#3421: dolt-server.port is not propagated into the worktree.
if [ -f "$root/.beads/dolt-server.port" ] && [ ! -e "$wt/.beads/dolt-server.port" ]; then
  mkdir -p "$wt/.beads"
  link=$(python3 -c 'import os,sys; print(os.path.relpath(sys.argv[1], sys.argv[2]))' \
    "$root/.beads/dolt-server.port" "$wt/.beads")
  ln -sf "$link" "$wt/.beads/dolt-server.port"
fi

# gastownhall/beads#3593: tighten .beads perms so bd stops warning.
[ -d "$wt/.beads" ] && chmod 700 "$wt/.beads"

# bd bases the new branch off the orchestrator tree's current HEAD, not origin/main.
git -C "$wt" reset --hard origin/main --quiet

echo "$wt"
