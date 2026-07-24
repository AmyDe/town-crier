#!/bin/bash
# PreToolUse hook: block code edits outside a git worktree.
# Ensures isolation when multiple conversations run in parallel.

file_path=$(jq -r '.tool_input.file_path // empty')
[ -z "$file_path" ] && exit 0

# Only gate source code files
case "$file_path" in
  *.go|*.swift|*.ts|*.tsx|*.css|*/.github/*.yml|*/.github/*.yaml) ;;
  *) exit 0 ;;
esac

# Worktrees live at <repo>/.claude/worktrees/<name>/, so project files inside one
# carry ".claude" in their path. Strip the worktree prefix before applying the
# exemptions below, so a real source file in a worktree is judged on its project
# path rather than on the worktree container.
rel="$file_path"
case "$file_path" in
  */.claude/worktrees/*) rel="/${file_path#*/.claude/worktrees/*/}" ;;
esac

# Skip non-project paths
case "$rel" in
  */.claude/*|*/node_modules/*|*/bin/*|*/obj/*|*/.beads/*) exit 0 ;;
esac

# Check 1: file path is under the worktree directory (the required location —
# EnterWorktree only auto-approves paths under .claude/worktrees/)
[[ "$file_path" == *"/.claude/worktrees/"* ]] && exit 0

# Check 2: session CWD is inside a linked worktree
# (In a linked worktree, --git-dir contains /worktrees/)
git_dir=$(git rev-parse --git-dir 2>/dev/null)
[[ "$git_dir" == *"/worktrees/"* ]] && exit 0

# Check 3: legacy layout — worktrees created before the .claude/worktrees/ rule
# sit at <repo>/<name>/ (bd's default when handed a bare name). Keep accepting them
# so in-flight work isn't stranded; new worktrees must still go under .claude/worktrees/.
# Use git worktree list to enumerate all linked worktrees and check if file_path falls
# under one of them (excluding the main worktree at the repo root).
main_tree=$(git rev-parse --show-toplevel 2>/dev/null)
while IFS= read -r line; do
  worktree_path=$(echo "$line" | awk '{print $1}')
  # Skip the main worktree
  if [ "$worktree_path" = "$main_tree" ]; then
    continue
  fi
  # Check if the file lives under this linked worktree
  if [[ "$file_path" == "$worktree_path"/* ]]; then
    exit 0
  fi
done < <(git worktree list 2>/dev/null)

jq -n '{
  "decision": "block",
  "reason": "Not in a worktree. Code changes must happen in an isolated worktree to avoid conflicts with parallel sessions.\n\nTo create and enter one:\n  1. Bash: scripts/wf/worktree-setup.sh <name> [--branch <branch>]\n     (creates it at <repo>/.claude/worktrees/<name> and prints the absolute path)\n  2. EnterWorktree path: \"<path printed by the command>\" to switch in.\n\nWorktrees MUST live under .claude/worktrees/ — EnterWorktree auto-approves those\npaths, and prompts unanswerable from settings for anywhere else. Never run bd\nworktree create with a bare name; its default <repo>/<name> layout triggers that\nprompt. Plain EnterWorktree name: / git worktree add trigger beads bug GH#3311."
}'
