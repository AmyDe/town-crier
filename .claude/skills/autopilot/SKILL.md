---
name: autopilot
description: "Autonomous single-bead worker loop. Picks one ready bead, dispatches the right engineer agent (dotnet-tdd-worker, ios-tdd-worker, react-tdd-worker, pulumi-infra-worker, github-actions-worker) in an isolated worktree, validates TDD evidence, creates a PR, watches CodeRabbit and CI gates, and either merges on green or updates the bead with failure context for retry. Designed for `/loop` — invoke every few minutes to continuously ship beads without supervision. Trigger on: 'autopilot', 'auto-work', 'work the backlog', 'pick up a bead', 'start the loop', 'ship beads automatically', or any request for autonomous bead processing from the ready queue."
---

# Autopilot

You are the **Town Crier's autopilot** — an autonomous loop that picks one ready bead, ships it through a complete PR lifecycle, and returns. Designed for `/loop` — each invocation handles exactly one bead, then exits so the loop can fire again.

## Each Invocation

```
Find work → Dispatch worker → Validate evidence → Create PR → Watch gates → Merge or follow up → Return
```

If no beads are ready, return immediately. The loop retries later.

## Phase 1: Find Work

Prune stale worktrees from prior runs:

```bash
git worktree prune
```

Invoke `/beads:ready` to find beads with no blockers.

**No beads ready?** Report `Autopilot: no ready beads — idle` and return.

**Beads available?** Walk the list from highest priority to lowest, selecting the first bead that passes all filters below. For each candidate, invoke `/beads:show <bead-id>` and apply these filters in order:

### Filter: Skip Epics

If the bead's type is `epic`, skip it — epics are containers, not implementable units. Continue to the next candidate.

### Filter: Retry Guard

If the bead's notes already contain **two or more** `Autopilot:` failure entries (from prior failed PR attempts), this bead has been tried and failed repeatedly. Do **not** retry it — instead:

1. Invoke `/beads:update <bead-id> --notes="Autopilot: flagged for human review after repeated failures"`.
2. Skip to the next candidate.

This prevents infinite retry loops. A human needs to look at it.

### Filter: Classify Worker Type

Determine the worker type from the bead's description and any labels:

| Signals | Worker |
|---------|--------|
| Swift, SwiftUI, iOS, mobile, XCTest, ViewModel, Coordinator, `mobile/ios` paths | `ios-tdd-worker` |
| .NET, C#, API, handler, endpoint, Cosmos, TUnit, `api` paths | `dotnet-tdd-worker` |
| React, TypeScript, web, CSS, frontend, Vite, Vitest, component, hook, `web` paths | `react-tdd-worker` |
| Pulumi, infrastructure, IaC, Azure, Container Apps, resource group, `infra` paths | `pulumi-infra-worker` |
| CI/CD, pipeline, GitHub Actions, workflow, deployment, `.github/workflows` paths | `github-actions-worker` |

Read the description carefully. If the bead cannot be mapped to any worker (e.g., manual tasks like App Store setup, or genuinely ambiguous), skip it and continue to the next candidate. Do not guess.

### No Candidate Found

If every ready bead was filtered out, report `Autopilot: no dispatchable beads — <N> ready but all filtered (epics, retry-limited, or unclassifiable)` and return.

## Phase 2: Dispatch

You now have a selected bead and its worker type from the filters above.

Mark the bead in-progress:

```
/beads:update <bead-id> --status=in_progress
```

Determine the worker's allowed path scope:

| Worker | Allowed Path |
|--------|-------------|
| `dotnet-tdd-worker` | `api/` |
| `ios-tdd-worker` | `mobile/ios/` |
| `react-tdd-worker` | `web/` |
| `pulumi-infra-worker` | `infra/` |
| `github-actions-worker` | `.github/workflows/`, `.github/actions/` |

Dispatch the worker with the critical requirements reinforced in the prompt:

```
Agent({
  "subagent_type": "<worker-type>",
  "name": "autopilot-worker",
  "description": "Implement bead <bead-id>",
  "isolation": "worktree",
  "model": "opus",
  "mode": "bypassPermissions",
  "run_in_background": true,
  "prompt": "Work on bead `<bead-id>`.\n\nCritical requirements:\n- Record a bead comment after EVERY Red and EVERY Green phase — this is your primary deliverable\n- Only modify files under `<allowed-path>` — do not touch anything outside this boundary\n- If you are unsure about scope or design, add a bead comment explaining the ambiguity and stop"
})
```

You are notified automatically when the worker completes — do not poll.

## Phase 3: Validate Evidence

The Agent result includes the **worktree path and branch name** if the worker made changes.

**No changes reported?** The worker failed silently. Jump to [Failure Recovery](#failure-recovery).

### Audit Checklist

Run every check below. If **any** check fails, jump to [Failure Recovery](#failure-recovery) with a specific description of what was missing.

#### Check 1: Commits Exist

```bash
git log main..<branch-name> --oneline
```

If no commits, fail with: "no commits on branch."

#### Check 2: Evidence Comments Exist

Invoke `/beads:show <bead-id>` and read the comments. Count the evidence comments recorded by the worker.

**For TDD workers (dotnet, ios, react):**
- Count comments containing `— Red` (failing test output)
- Count comments containing `— Green` (passing test output)
- There must be **at least one Red comment and at least one Green comment** — a single summary with no per-cycle evidence is not sufficient
- There must be a **summary comment** containing `## TDD Summary` with final test output

If Red comments = 0, fail with: "no Red phase evidence found — worker may not have followed TDD."
If Green comments = 0, fail with: "no Green phase evidence found."
If summary is missing, fail with: "no TDD Summary comment found."

**For infra worker:**
- There must be at least one comment containing `## Infrastructure Change`
- There must be a summary comment containing `## Infrastructure Summary`

**For CI/CD worker:**
- There must be at least one comment containing `## Pipeline Change`
- There must be a summary comment containing `## Pipeline Summary`

#### Check 3: Commit Count Plausibility

For TDD workers, count commits on the branch and count Red-Green pairs in comments:

```bash
git log main..<branch-name> --oneline | wc -l
```

There should be at least 2 commits (one Red, one Green). If there's only 1 commit, fail with: "only 1 commit found — TDD requires at least one Red and one Green commit."

#### Check 4: Scope Verification

Check that all changed files fall within the worker's allowed path boundary:

```bash
git diff main..<branch-name> --name-only
```

| Worker | Allowed Paths |
|--------|--------------|
| `dotnet-tdd-worker` | `api/` |
| `ios-tdd-worker` | `mobile/ios/` |
| `react-tdd-worker` | `web/` |
| `pulumi-infra-worker` | `infra/` |
| `github-actions-worker` | `.github/` |

If any files fall outside the allowed paths, fail with: "files modified outside scope: `<list of offending files>`."

#### All Checks Pass

Continue to Phase 4.

## Phase 4: Create Pull Request

Push the branch to the remote:

```bash
git push -u origin <branch-name>
```

Get the repo owner/name for API calls:

```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner'
```

Create the PR:

```bash
gh pr create \
  --head <branch-name> \
  --base main \
  --title "<bead title (keep under 70 chars)>" \
  --body "$(cat <<'EOF'
## Summary

Implements `<bead-id>`: <bead title>

<one-line description of what changed>

## Bead

`<bead-id>` — see bead comments for TDD evidence.

---
Shipped by Town Crier autopilot
EOF
)"
```

Record the PR number from the output.

## Phase 5: Watch Gates

Wait for CI checks to complete:

```bash
gh pr checks <pr-number> --watch 2>&1
```

Capture the output and exit code. Notes:
- If no checks are configured, `gh pr checks` returns immediately — that's fine, proceed.
- Do **not** use `--fail-fast` — wait for all checks so you can report the full picture.
- If the command hangs beyond 15 minutes, treat as a timeout failure.

### CodeRabbit Review

After CI checks resolve, fetch CodeRabbit's review:

```bash
gh api repos/<owner>/<repo>/pulls/<pr-number>/reviews \
  --jq '.[] | select(.user.login == "coderabbitai") | .body' 2>/dev/null

gh api repos/<owner>/<repo>/pulls/<pr-number>/comments \
  --jq '.[] | select(.user.login == "coderabbitai") | .body' 2>/dev/null
```

If CodeRabbit hasn't reviewed yet (empty results), wait 30 seconds and retry once. If still no review, proceed without it — CodeRabbit may not be configured.

### Classify Comments

Read all CodeRabbit comments and classify each:

**Nitpicks** (merge anyway):
- Style or formatting preferences
- Naming suggestions where both options are valid
- Minor documentation wording
- Comments CodeRabbit labels as "nitpick" or "suggestion"
- "Consider using X" where both approaches are correct

**Substantive** (block merge):
- Bugs or logic errors
- Security vulnerabilities
- Missing error handling for real failure modes
- Architectural violations
- Missing test coverage for important paths
- Performance issues with measurable impact

When in doubt, treat as substantive. Better to retry than to merge a real problem.

## Phase 6: Resolve

### Path A: All Green — Merge

All CI checks pass **and** no substantive CodeRabbit comments.

```bash
gh pr merge <pr-number> --squash --delete-branch
```

Close the bead:

```
/beads:close <bead-id>
```

Clean up and sync:

```bash
git pull --rebase
git worktree prune
```

Sync beads to remote (no plugin skill for Dolt sync — use CLI):

```bash
bd dolt push
```

Report: `Autopilot: merged PR #<number> for <bead-id>: <title>`

### Path B: Gates Failed — Retry Later

CI checks failed **or** substantive CodeRabbit comments found.

**Step 1: Collect failure details**

For CI failures:
```bash
gh pr checks <pr-number> 2>&1
```

For CodeRabbit: note the substantive comments from Phase 5.

**Step 2: Close the PR and clean up the branch**

```bash
gh pr close <pr-number>
git push origin --delete <branch-name>
git worktree prune
```

**Step 3: Update the bead for retry**

Add the failure context to the bead so the next worker attempt can address it:

```
/beads:update <bead-id> --status=open
/beads:update <bead-id> --notes="Autopilot: PR #<number> failed. <date>

CI failures:
<failed check names and brief error descriptions>

CodeRabbit issues:
<substantive comments, quoted>

The next implementation attempt should address these issues."
```

The bead is now `open` again with failure context in its notes. The next autopilot cycle will pick it up, and the worker will see the notes when it reads the bead via `/beads:show`.

**Step 4: Sync beads** (no plugin skill for Dolt sync — use CLI)

```bash
bd dolt push
```

Report: `Autopilot: PR #<number> for <bead-id> failed — bead updated with failure context for retry`

---

## Failure Recovery

Used when the worker fails before a PR is created (no evidence, no commits, silent failure).

```
/beads:update <bead-id> --status=open
/beads:update <bead-id> --notes="Autopilot: worker failed to produce evidence. <date>. <brief description of what went wrong if known>"
```

```bash
git worktree prune
```

Sync beads (no plugin skill for Dolt sync — use CLI):

```bash
bd dolt push
```

Report: `Autopilot: worker failed for <bead-id> — bead reset to open for retry`

## Rules

- **One bead per invocation.** Pick one, ship it, return. The loop handles repetition.
- **Never write code.** Worker agents handle all implementation. You orchestrate.
- **Never skip evidence validation.** No PR without verified test/build evidence on the bead.
- **Always use PRs.** Never push directly to main. PRs provide gate enforcement and review.
- **Respect CodeRabbit.** Only ignore comments that are genuinely nitpicks. When in doubt, treat as substantive and retry.
- **Two-strike retry limit.** If a bead has failed twice already (two `Autopilot:` failure entries in notes), flag it for human review instead of retrying.
- **Always `bd dolt push` after any bead state change.** No plugin skill for Dolt sync — use CLI directly. Sync immediately, not at session end.
- **Always clean up.** Prune worktrees and delete remote branches after every cycle.
- **Return fast when idle.** No beads = return immediately. Don't wait or poll.
- **Fail safe.** Unexpected errors → reset bead to open → clean up → return. The next cycle retries.
- **Use `/beads:*` skills for all tracking.** No TodoWrite, no TaskCreate.
