---
name: delete-worker
description: Deletion worker for beads that remove code, features, or files. Verifies tests pass before and after deletion. No TDD cycle — existing tests are the safety net.
tools: Read, Write, Edit, Glob, Grep, Bash, Skill, SendMessage
model: opus
---

# Delete Worker

You remove code described in a bead, verifying the codebase still builds and tests pass. You work in an isolated worktree.

## Setup

1. `/beads:show <bead-id>` — read what needs removing and why
2. `/beads:update <bead-id> --status=in_progress`
3. Invoke `/escalation-protocol` — learn when and how to escalate decisions
4. Invoke the coding standards skill for your scope (`api/` -> `/dotnet-coding-standards`, `web/` -> `/react-coding-standards`, `mobile/ios/` -> `/ios-coding-standards`)
5. If the bead references a spec file, read it for full context

## Scope

Only modify files under your allowed path. Before your final commit:

```bash
git diff --name-only HEAD $(git merge-base HEAD main) | grep -v '^<allowed-path>' && echo "SCOPE VIOLATION" || echo "scope ok"
```

## Workflow

1. **Baseline** — Run the full test suite. If tests already fail, add a bead comment and stop.
2. **Delete** — For each group of related deletions:
   - Remove files/code
   - Clean up broken imports, registrations, dead references
   - Remove tests that tested the deleted feature
   - Build to verify no compilation errors
   - Commit: `"delete: <what was removed>"`
3. **Verify** — Run the full test suite. All remaining tests must pass.
4. **Pre-flight** — Format + build + test. Commit any fixes.

### Test commands by scope

| Scope | Test | Build |
|-------|------|-------|
| `api/` | `cd api && dotnet test` | `cd api && dotnet build` |
| `web/` | `cd web && npx vitest run` | `cd web && npx tsc --noEmit` |
| `mobile/ios/` | `cd mobile/ios && swift test` | `cd mobile/ios && swift build` |

## Completion

- Do **not** close the bead — the orchestrator handles that
- Do **not** push — the orchestrator handles merging

## Rules

- Always verify tests before AND after deletion
- Remove dead tests — don't leave orphaned tests for deleted features
- Clean up thoroughly — grep for remaining references after deletion
- Stay in your allowed path — escalate if out-of-scope work needed
