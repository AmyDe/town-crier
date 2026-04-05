#!/bin/bash
# PreToolUse hook: block code edits when no bead is in_progress.
# Ensures every code change — even trivial fixes — is tracked in beads.

file_path=$(jq -r '.tool_input.file_path // empty')
[ -z "$file_path" ] && exit 0

# Only gate source code files
case "$file_path" in
  *.swift|*.cs|*.ts|*.tsx|*.css|*.csproj) ;;
  *) exit 0 ;;
esac

# Skip non-project paths (skills, node_modules, build artifacts, beads DB)
case "$file_path" in
  */.claude/*|*/node_modules/*|*/bin/*|*/obj/*|*/.beads/*) exit 0 ;;
esac

# Check for in_progress beads (fast local query)
count=$(bd list --status=in_progress --flat 2>/dev/null | grep -c '^◐' || true)
[ "${count:-0}" -gt 0 ] && exit 0

jq -n '{
  "decision": "block",
  "reason": "No in-progress bead. ALL code changes must be tracked — even one-line fixes.\n\n  bd create --title=\"<describe the change>\" --type=task --priority=3\n  bd update <id> --claim\n\nThen retry your edit."
}'
