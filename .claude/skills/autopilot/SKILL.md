---
name: autopilot
description: "Autonomous single-bead worker loop. Picks a ready bead, dispatches the right engineer agent in an isolated worktree, validates TDD evidence, and merges the result. Designed for `/loop` — invoke every few minutes to continuously ship beads without supervision. Trigger on: 'autopilot', 'auto-work', 'work the backlog', 'pick up a bead', 'start the loop', 'ship beads automatically', or any request for autonomous bead processing from the ready queue."
---

# Autopilot

You are the **Town Crier's autopilot** — an autonomous loop that picks one ready bead, ships it via a worker agent, and merges the result into a session branch. Designed for `/loop` — each invocation handles exactly one bead, then exits so the loop can fire again.

## Each Invocation

```
Ensure session branch → Find work → Dispatch worker → Validate evidence → Merge to session branch → Return
```

If no beads are ready, return immediately. The loop retries later.

## Phase 0: Ensure Session Branch

The autopilot always works on a branch — never directly on main. All worker output accumulates on a single session branch. The user creates a PR and merges manually the next morning.

### If on `main`

Create a new session branch with an ISO 8601 timestamp (colons removed for branch-name compatibility):

```bash
git pull --rebase
branch_name="autopilot/$(date -u +%Y-%m-%dT%H%M%SZ)"   # e.g. autopilot/2026-03-29T210435Z
git checkout -b "$branch_name"
```

Every autopilot invocation gets a fresh branch — never reuse an existing one.

### If already on a branch

Verify it's an `autopilot/` branch. If so, continue using it. If it's some other branch, **do not switch** — report `Autopilot: on unexpected branch <name> — expected main or autopilot/*. Returning.` and exit.

### Push the branch

Ensure the branch is tracked on the remote:

```bash
git push -u origin HEAD
```

## Phase 1: Find Work

Clean up orphaned worktrees from prior runs. Log each removal so the user can see what was cleaned:

```bash
for wt in $(git worktree list --porcelain | grep '^worktree ' | awk '{print $2}' | grep '\.claude/worktrees/'); do
  uncommitted=$(git -C "$wt" status --short 2>/dev/null)
  if [ -n "$uncommitted" ]; then
    echo "Autopilot: warning — orphaned worktree $wt has uncommitted changes:"
    echo "$uncommitted"
  fi
  echo "Autopilot: removing orphaned worktree $wt"
  bd worktree remove "$wt" --force 2>/dev/null || git worktree remove --force "$wt" 2>/dev/null
done
git worktree prune
```

Invoke `/beads:ready` to find beads with no blockers.

**No beads ready?** Report `Autopilot: no ready beads — idle` and return.

**Beads available?** Walk the list from highest priority to lowest, selecting the first bead that passes all filters below. For each candidate, invoke `/beads:show <bead-id>` and apply these filters in order.

> **Note:** Candidates are evaluated one at a time because each requires a `/beads:show` call. If many beads are filtered out (epics, unclassifiable), this adds up. Keep bead hygiene tight to minimize wasted cycles.

### Filter: Skip Epics

If the bead's type is `epic`, skip it — epics are containers, not implementable units. Continue to the next candidate.

### Filter: Classify Worker Type

Determine the worker type from the bead's description, labels, and any paths mentioned:

| Signals | Worker |
|---------|--------|
| delete, remove, strip, clean up, drop (feature/code removal tasks) + any tech area signals below | `delete-worker` |
| Swift, SwiftUI, iOS, mobile, XCTest, ViewModel, Coordinator, `mobile/ios` paths | `ios-tdd-worker` |
| .NET, C#, API, handler, endpoint, Cosmos, TUnit, `api` paths | `dotnet-tdd-worker` |
| React, TypeScript, web, CSS, frontend, Vite, Vitest, component, hook, `web` paths | `react-tdd-worker` |
| Pulumi, infrastructure, IaC, Azure, Container Apps, resource group, `infra` paths | `pulumi-infra-worker` |
| CI/CD, pipeline, GitHub Actions, workflow, deployment, `.github/workflows` paths | `github-actions-worker` |

**Delete worker classification:** If the bead's primary intent is removing/deleting code or features (not adding or modifying), classify as `delete-worker` regardless of tech area. The delete worker's allowed path is determined by the same tech area signals — e.g., a bead saying "remove Groups feature from web" maps to `delete-worker` with allowed path `web/`.

If the bead cannot be mapped to any worker (e.g., manual tasks, genuinely ambiguous), **mark it blocked**:

```
/beads:update <bead-id> --status=blocked --append-notes="Autopilot: cannot classify worker type. <date>. Needs human triage."
```

Continue to the next candidate.

### No Candidate Found

If no workable candidate was found, report `Autopilot: no dispatchable beads — idle` and return.

### Claim Immediately

Once a candidate passes all filters, mark it in-progress **before** anything else — this prevents a concurrent `/loop` invocation from selecting the same bead:

```
/beads:update <bead-id> --status=in_progress
```

## Phase 2: Dispatch

You now have a claimed bead and its worker type.

Determine the worker's allowed path scope (this table is the single source of truth — also used in Check 4 during validation):

| Worker | Allowed Path |
|--------|-------------|
| `dotnet-tdd-worker` | `api/` |
| `ios-tdd-worker` | `mobile/ios/` |
| `react-tdd-worker` | `web/` |
| `pulumi-infra-worker` | `infra/` |
| `github-actions-worker` | `.github/workflows/`, `.github/actions/` |
| `delete-worker` | *(determined by tech area signals — same as the TDD/infra worker it replaces)* |

Create a worktree using `bd worktree create`, which sets up a `.beads/redirect` file so `bd` commands in the worktree transparently use the main repo's database — no environment variables needed:

```bash
worktree_name="autopilot-<bead-id>"
worktree_branch="autopilot/<bead-id>"
bd worktree create ".claude/worktrees/$worktree_name" --branch "$worktree_branch"
worktree_path="$(pwd)/.claude/worktrees/$worktree_name"
```

Store `worktree_path` and `worktree_branch` — you need them for validation, merge, and cleanup.

Dispatch the worker into the pre-created worktree (no `isolation` — the worktree already exists):

**For TDD and infra/CI workers:**

```
Agent({
  "subagent_type": "<worker-type>",
  "name": "autopilot-worker",
  "description": "Implement bead <bead-id>",
  "model": "opus",
  "mode": "bypassPermissions",
  "run_in_background": true,
  "prompt": "Work on bead `<bead-id>`.\n\nYou are working in a pre-created worktree at `<worktree_path>`. All your commands must run from this directory — prefix every Bash call with `cd <worktree_path> &&`.\n\nThe worktree has beads configured via redirect — `bd` commands work automatically, no setup needed.\n\nCritical requirements:\n- Record a bead comment after EVERY Red and EVERY Green phase — this is your primary deliverable\n- Only modify files under `<allowed-path>` — do not touch anything outside this boundary\n- If you are unsure about scope or design, add a bead comment explaining the ambiguity and stop\n- NEVER run `bd init`, `bd init --force`, or `bd doctor --fix` — these destroy the shared beads database. If bd commands fail, add a bead comment describing the error and continue with code work."
})
```

**For delete worker:**

```
Agent({
  "subagent_type": "delete-worker",
  "name": "autopilot-worker",
  "description": "Delete code for bead <bead-id>",
  "model": "opus",
  "mode": "bypassPermissions",
  "run_in_background": true,
  "prompt": "Work on bead `<bead-id>`.\n\nYou are working in a pre-created worktree at `<worktree_path>`. All your commands must run from this directory — prefix every Bash call with `cd <worktree_path> &&`.\n\nThe worktree has beads configured via redirect — `bd` commands work automatically, no setup needed.\n\nYour allowed path is `<allowed-path>`.\n\nCritical requirements:\n- Run tests BEFORE and AFTER deletions to prove nothing broke\n- Record a bead comment for every deletion group — this is your primary deliverable\n- Only modify files under `<allowed-path>` — do not touch anything outside this boundary\n- If you are unsure about scope or design, add a bead comment explaining the ambiguity and stop\n- NEVER run `bd init`, `bd init --force`, or `bd doctor --fix` — these destroy the shared beads database. If bd commands fail, add a bead comment describing the error and continue with code work."
})
```

You are notified automatically when the worker completes — do not poll.

## Phase 3: Validate Evidence

You already know the `worktree_path` and `worktree_branch` from Phase 2 (created before dispatch).

**Worker reported no changes?** Check whether the worktree branch has commits anyway (`git log main..<worktree_branch>`). If truly empty, jump to [Failure](#failure).

### Audit Checklist

Run every check below. If **any** check fails, jump to [Failure](#failure) with a specific description of what was missing.

#### Check 1: Commits Exist

```bash
git log main..<worker-branch> --oneline
```

If no commits, fail with: "no commits on branch."

#### Check 2: Evidence Comments Exist

Invoke `/beads:show <bead-id>` and read the comments.

**For TDD workers (dotnet, ios, react):**
- Count comments containing `— Red` (failing test output)
- Count comments containing `— Green` (passing test output)
- There must be **at least one Red comment and at least one Green comment**
- There must be a **summary comment** containing `## TDD Summary`

If Red = 0, fail: "no Red phase evidence."
If Green = 0, fail: "no Green phase evidence."
If summary missing, fail: "no TDD Summary comment."

**For delete worker:**
- At least one comment containing `## Baseline` (pre-deletion test run)
- At least one comment containing `## Deletion:` (what was removed + build verification)
- A summary comment containing `## Deletion Summary`

If baseline missing, fail: "no Baseline evidence."
If deletion comments = 0, fail: "no Deletion evidence."
If summary missing, fail: "no Deletion Summary comment."

**For infra worker:**
- At least one comment containing `## Infrastructure Change`
- A summary comment containing `## Infrastructure Summary`

**For CI/CD worker:**
- At least one comment containing `## Pipeline Change`
- A summary comment containing `## Pipeline Summary`

#### Check 3: Commit Count Plausibility

For TDD workers only (skip this check for delete, infra, and CI/CD workers):

```bash
git log main..<worker-branch> --oneline | wc -l
```

At least 2 commits required (one Red, one Green). If only 1, fail: "only 1 commit — TDD requires at least Red and Green."

#### Check 4: Scope Verification

```bash
git diff main..<worker-branch> --name-only
```

All changed files must fall within the worker's allowed path (see the table in Phase 2 — that's the single source of truth). If any are outside, fail: "files modified outside scope: `<list>`."

#### All Checks Pass

Continue to Phase 4.

## Phase 4: Merge to Session Branch

The worker produced validated work on its worktree branch. Verify the branch is available, then merge:

```bash
git branch --list <worker-branch>   # verify it exists in local refs
git merge <worker-branch> --no-edit
```

If the merge has conflicts, this is a failure — jump to [Failure](#failure) with: "merge conflict with session branch."

Push the updated session branch:

```bash
git push
```

Close the bead:

```
/beads:close <bead-id>
```

Clean up the worktree:

```bash
bd worktree remove <worktree-path> --force
git worktree prune
```

Sync beads:

```bash
bd dolt push
```

Build a session tally by counting commits on the session branch and blocked beads:

```bash
merged_count=$(git log main..HEAD --oneline | wc -l | tr -d ' ')
blocked_count=$(bd list --status=blocked 2>/dev/null | grep -c '^beads-' || echo 0)
remaining=$(bd ready 2>/dev/null | grep -c '^beads-' || echo 0)
```

Report with the tally:

```
Autopilot: merged <bead-id> — <title>
Session: <merged_count> commits, <blocked_count> blocked | <remaining> beads remaining | branch: <current-branch>
```

If `remaining` is 0, add: `All beads closed. Run /ship to PR and merge the session branch.`

## Failure

Used when the worker fails, evidence validation fails, or merge fails. **One strike — mark blocked and move on.**

1. Record what went wrong:

```
/beads:update <bead-id> --status=blocked --append-notes="Autopilot: failed <date>. Reason: <specific failure description>"
```

2. Clean up the worktree (if one exists):

```bash
bd worktree remove <worktree-path> --force 2>/dev/null || git worktree remove --force <worktree-path> 2>/dev/null
git worktree prune
```

3. Sync beads:

```bash
bd dolt push
```

4. Report: `Autopilot: <bead-id> blocked — <reason>`

The bead will not appear in `bd ready` again. The user reviews blocked beads in the morning.

## Rules

- **One bead per invocation.** Pick one, ship it, return. The loop handles repetition.
- **Never write code.** Worker agents handle all implementation. You orchestrate.
- **Never skip evidence validation.** No merge without verified evidence on the bead.
- **Never commit to main.** All work goes to the session branch.
- **One strike.** If a bead fails for any reason, mark it blocked immediately. No retries.
- **Always use `--append-notes`** when adding failure context — never `--notes`, which overwrites.
- **Always `bd dolt push` after any bead state change.**
- **Always clean up.** Remove worktrees with `bd worktree remove <path> --force` and prune.
- **Return fast when idle.** No beads = return immediately.
- **Fail safe.** Unexpected errors → mark bead blocked → clean up → return.
- **Use `/beads:*` skills for all tracking.** No TodoWrite, no TaskCreate.
