---
name: autopilot
description: "Autonomous single-bead worker loop. Picks a ready bead, dispatches the right worker in an isolated worktree, validates via post-merge tests, and merges the result. Designed for `/loop`. Trigger on: 'autopilot', 'auto-work', 'work the backlog', 'pick up a bead', 'start the loop', 'ship beads automatically'."
---

# Autopilot

Autonomous loop: pick one ready bead, ship it, merge it, return. Designed for `/loop`.

```
Session branch -> Find work -> Dispatch worker -> Merge -> Verify tests -> Close -> Return
```

## Phase 0: Session Branch

Always work on a branch, never main.

**On `main`:**
```bash
# If the working tree is dirty (typically auto-exported .beads/issues.jsonl from a
# prior session's bd mutations), stash instead of committing. Creating a standalone
# `chore(beads): sync` commit here causes rebase conflicts at the end of ship, because
# squash-merge of the PR will replay a different jsonl snapshot on top of it.
if ! git diff-index --quiet HEAD --; then
  git stash -u -m "autopilot-phase0"
  _stashed=1
fi
git pull --rebase
git checkout -b "autopilot/$(date -u +%Y-%m-%dT%H%M%SZ)"
git push -u origin HEAD
# Drop the stash — it's pre-mutation bd state that the worker's own commits will supersede.
if [ "$_stashed" = "1" ]; then git stash drop; fi
```

**On `autopilot/*`:** Continue using it.

**On other branch:** Report error and exit.

## Phase 1: Find Work

Clean orphaned worktrees:
```bash
# Use bd worktree list to find orphans, bd worktree remove to clean them
bd worktree list --json | jq -r '.[] | select(.path | contains(".claude/worktrees/")) | .path' | while read wt; do
  echo "Autopilot: removing orphaned worktree $wt"
  bd worktree remove "$wt" --force
done
```

Invoke `/beads:ready`. No beads? **Stop the loop:**

1. **Cancel cron job (if any):** Run `CronList`. If a job exists, `CronDelete` it.
2. **Do NOT call `ScheduleWakeup`.** Omitting the call ends a dynamic loop.
3. Report `Autopilot: no ready beads — loop stopped.` and return.

Walk highest priority first. For each candidate, `/beads:show`:

- **Skip epics** — containers, not implementable
- **Classify worker type:**

| Signals | Worker | Allowed Path |
|---------|--------|-------------|
| delete/remove/strip + tech signals | `delete-worker` | *(by tech area)* |
| Swift, iOS, mobile, `mobile/ios` | `ios-tdd-worker` | `mobile/ios/` |
| .NET, C#, API, handler, `api` | `dotnet-tdd-worker` | `api/` |
| React, TypeScript, web, `web` | `react-tdd-worker` | `web/` |
| Pulumi, infra, Azure, `infra` | `pulumi-infra-worker` | `infra/` |
| CI/CD, GitHub Actions, `.github` | `github-actions-worker` | `.github/` |

**Unclassifiable?** Mark blocked: `/beads:update <id> --status=blocked --append-notes="Autopilot: cannot classify. Needs human triage."`

**Found candidate?** Claim: `/beads:update <id> --status=in_progress`

## Phase 2: Dispatch

Create worktree:
```bash
bd worktree create ".claude/worktrees/autopilot-<bead-id>" --branch "autopilot/<bead-id>"
worktree_path="$(pwd)/.claude/worktrees/autopilot-<bead-id>"
```

Dispatch:
```
Agent({
  "subagent_type": "<worker-type>",
  "name": "autopilot-worker",
  "description": "Implement bead <bead-id>",
  "model": "opus",
  "mode": "bypassPermissions",
  "run_in_background": true,
  "prompt": "Work on bead `<bead-id>`.\n\nYou are in a pre-created worktree at `<worktree_path>`. Prefix Bash calls with `cd <worktree_path> &&`.\n\nBeads configured via redirect — `bd` commands work automatically.\nOnly modify files under `<allowed-path>`.\nNEVER run `bd init`, `bd init --force`, or `bd doctor --fix`."
})
```

## Phase 3: Validate and Merge

When the worker completes:

### Check 1: Commits exist
```bash
git log main..<worker-branch> --oneline
```
No commits -> failure.

### Check 2: Scope
```bash
git diff main..<worker-branch> --name-only
```
All files must be under the allowed path. Any outside -> failure.

### Check 3: Merge
```bash
# Stash any auto-exported .beads/issues.jsonl / .gitignore churn first — those files
# get re-touched by bd worktree create / bd close in the parent tree during Phase 2,
# and block the merge even though they're not conflicts.
git stash -u -m "autopilot-phase3" 2>/dev/null
git merge <worker-branch> --no-edit
# Drop the stash — worker's commits (and the `merge=theirs` driver on .beads/issues.jsonl)
# are now authoritative.
git stash drop 2>/dev/null
```
Conflicts -> `git merge --abort` -> failure.

### Check 4: Tests pass

Run the relevant test suite on the merged result:

| Worker | Test Command |
|--------|-------------|
| `dotnet-tdd-worker` | `cd api && dotnet test` |
| `ios-tdd-worker` | `cd mobile/ios && swift test` |
| `react-tdd-worker` | `cd web && npx vitest run` |
| `pulumi-infra-worker` | `cd infra && dotnet build` |
| `github-actions-worker` | YAML validation only |
| `delete-worker` | *(same as tech area)* |

Tests fail -> `git reset --hard HEAD~1` -> failure.

### All pass

```bash
git push
```

Close and sync:
```
/beads:close <bead-id>
bd dolt push
```

Clean up:
```bash
bd worktree remove <worktree-path> --force
```

Report:
```
Autopilot: merged <bead-id> — <title>
Session: <N> commits, <N> blocked | <N> remaining | branch: <name>
```

If 0 remaining: `All beads closed. Run /ship to PR and merge.`

## Failure

One strike — mark blocked and move on:

```
/beads:update <bead-id> --status=blocked --append-notes="Autopilot: failed <date>. Reason: <specific>"
```

Clean up worktree, sync beads (`bd dolt push`), report: `Autopilot: <bead-id> blocked — <reason>`

## Rules

- **One bead per invocation.** Pick, ship, return. The loop handles repetition.
- **Never write code.** Workers handle implementation.
- **Never skip test verification.** Tests must pass after merge — this is the real evidence.
- **Never commit to main.** Session branch only.
- **One strike.** Failure -> block immediately, no retries.
- **Always `--append-notes`** (never `--notes`, which overwrites).
- **Always `bd dolt push`** after any bead state change.
- **Return fast when idle.**
