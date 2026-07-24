#!/bin/bash
# PreToolUse hook: block EnterWorktree's "create new" path.
# Allow "enter existing" (path:) — so agents can enter a bd-created worktree.
#
# Why: Beads bug gastownhall/beads#3311 makes plain `git worktree add` (which
# EnterWorktree uses under the hood) leave a stray `issues.jsonl` at the repo
# root on every commit. `bd worktree create` configures the worktree to share
# the main repo's beads database via git common-directory discovery and avoids
# the bug.

input=$(cat)
path=$(echo "$input" | jq -r '.tool_input.path // empty')

# path: set → entering an existing worktree. Allow.
if [ -n "$path" ]; then
  exit 0
fi

# name: set or nothing → creating a new worktree. Block.
jq -n '{
  "decision": "block",
  "reason": "EnterWorktree without path: is disabled in this project.\n\nbd-managed worktrees are required — plain `git worktree add` triggers beads bug GH#3311 (stray root `issues.jsonl`).\n\nTo create a worktree:\n  1. Bash: scripts/wf/worktree-setup.sh <name> [--branch <branch>]\n     (it runs `bd worktree create .claude/worktrees/<name>`, applies the bd\n     workarounds, rebases on origin/main, and prints the absolute path)\n  2. EnterWorktree path: \"<the path it printed>\" to switch into it.\n\nWorktrees MUST live under <repo>/.claude/worktrees/. EnterWorktree auto-approves\nthose paths; anywhere else it raises a permission prompt that settings cannot\npre-approve, which stalls unattended sessions. Never pass bd a bare name — its\ndefault <repo>/<name> layout is what triggers the prompt.\n\nTo enter a worktree that already exists, use EnterWorktree with path: instead of name:."
}'
