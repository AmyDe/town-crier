#!/bin/bash
# PreToolUse hook: block code edits outside a git worktree.
# Ensures isolation when multiple conversations run in parallel.

file_path=$(jq -r '.tool_input.file_path // empty')
[ -z "$file_path" ] && exit 0

# Only gate source code files
case "$file_path" in
  *.swift|*.cs|*.ts|*.tsx|*.css|*.csproj) ;;
  *) exit 0 ;;
esac

# Skip non-project paths
case "$file_path" in
  */.claude/*|*/node_modules/*|*/bin/*|*/obj/*|*/.beads/*) exit 0 ;;
esac

# Check 1: file path is under a worktree directory
[[ "$file_path" == *"/.claude/worktrees/"* ]] && exit 0

# Check 2: session CWD is inside a linked worktree
# (In a linked worktree, --git-dir contains /worktrees/)
git_dir=$(git rev-parse --git-dir 2>/dev/null)
[[ "$git_dir" == *"/worktrees/"* ]] && exit 0

jq -n '{
  "decision": "block",
  "reason": "Not in a worktree. Code changes must happen in an isolated worktree to avoid conflicts with parallel sessions.\n\nRun EnterWorktree to create one, then retry your edit."
}'
