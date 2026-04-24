---
name: autopilot
description: "Autonomous single-bead worker loop. Picks a ready bead, dispatches the right worker in an isolated worktree, validates via post-merge tests, and merges the result. Designed for `/loop`. Trigger on: 'autopilot', 'auto-work', 'work the backlog', 'pick up a bead', 'start the loop', 'ship beads automatically'."
---

# Autopilot

Autonomous loop: pick one ready bead, ship it, merge it, return. Designed for `/loop`.

```
Session branch -> Find work -> [Split multi-worker?] -> Dispatch worker -> Merge -> Verify tests -> Close -> Return
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
# `bd worktree remove` takes a NAME, not a path. Passing a path fails with
# "cannot remove main repository as worktree" because bd can't resolve it and
# falls back to the main repo. Filter out is_main and select .name.
bd worktree list --json | jq -r '.[] | select(.is_main == false) | select(.path | contains(".claude/worktrees/")) | .name' | while read name; do
  echo "Autopilot: removing orphaned worktree $name"
  bd worktree remove "$name" --force
done
```

Invoke `/beads:ready`. Found work? Proceed to the classification table below.

**No ready beads?** Do NOT reflexively stop the loop — first check whether anything is still in-flight:

```bash
bd list --status=in_progress --json | jq 'length'
```

- **In-progress count > 0:** A worker is still running (or a prior run left a claimed bead that will close soon). When it closes, dependent beads will unblock. Cancelling the cron now forces the user to manually restart the loop the moment work reappears — cheap empty ticks every 5 min beat that friction every time.
  - **Keep the cron alive.** Do NOT call `CronDelete`.
  - **Do NOT call `ScheduleWakeup`.**
  - Report `Autopilot: no ready beads yet — N in-progress, loop continues.` and return.
- **In-progress count = 0:** The session is fully idle. Stop the loop:
  1. **Cancel cron job (if any):** Run `CronList`. If a job exists, `CronDelete` it.
  2. **Do NOT call `ScheduleWakeup`.** Omitting the call ends a dynamic loop.
  3. Report `Autopilot: no ready beads — loop stopped.` and return.

Walk highest priority first. For each candidate, `/beads:show` and read the bead fully — title, description, acceptance, design notes, and any referenced spec file (e.g. `docs/specs/<topic>.md#phase-1`):

- **Skip epics** — containers, not implementable.
- **Multi-worker scope? Split, don't classify.** If the bead's scope requires more than one worker type (e.g. `/api` endpoint + `/mobile/ios` consumer + `/web` consumer, or code + `/infra`), jump to **Phase 1.5: Split Multi-Worker Beads** instead of classifying.
- **Classify worker type** (single-worker beads only):

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

## Phase 1.5: Split Multi-Worker Beads

When a bead's scope requires more than one worker type — e.g. `/api` endpoint + `/mobile/ios` consumer + `/web` consumer, or code + `/infra` — split it into children, one per worker type, instead of claiming.

**Split when:**
- The spec or description lists work across 2+ areas (`/api`, `/mobile/ios`, `/web`, `/infra`, `.github`).
- A migration touches both code and infra/CI.

**Do NOT split (fall through to the normal single-worker flow):**
- Signals point to exactly one worker.
- Multiple tech areas are mentioned but the actual work is only in one (e.g. "fix iOS bug triggered by an earlier API change" — only iOS work to do).

**Ambiguous which files belong to which worker?** Don't guess — mark the parent blocked for human triage:

```bash
/beads:update <parent-id> --status=blocked --append-notes="Autopilot: multi-worker split ambiguous. Needs human triage."
bd dolt push
```

Then return.

### 1. Create a child bead per worker type

For each worker type needed:

```bash
bd create \
  --title="<parent title> — <area>" \
  --description="<slice of the parent's work, referencing the parent by ID>" \
  --type=<same as parent> \
  --priority=<same as parent> \
  --acceptance="<slice-scoped acceptance>" \
  --notes="Split from <parent-id> by autopilot. Worker: <worker-type>. Allowed path: <path>."
```

Title convention: `<parent title> — API`, `<parent title> — iOS`, `<parent title> — web`, `<parent title> — infra`, etc. Type, priority, and any spec-file references in the parent carry into each child.

### 2. Supersede the parent

```bash
bd supersede <parent-id> --with=<child-1-id>
# bd supersede takes one --with; record additional children in notes:
bd update <parent-id> --append-notes="Also superseded by: <child-2-id>, <child-3-id>"
bd update <child-2-id> --append-notes="Split sibling of <child-1-id>. Parent: <parent-id>."
bd update <child-3-id> --append-notes="Split sibling of <child-1-id>. Parent: <parent-id>."
```

### 3. Add dependencies (producer → consumer)

If one child produces a contract the other consumes, add a dep. Consumer depends on producer:

```bash
bd dep add <consumer-child-id> <producer-child-id>
```

Heuristics, in order:

1. **Contract producers before consumers:**
   - `.NET` / API → iOS, web (endpoint before its clients).
   - Pulumi / infra → anything that deploys into it.
   - GitHub Actions → code that relies on a new workflow or secret.
2. **Delete after migrate:** a `delete-worker` child always depends on any non-delete siblings.
3. **Otherwise parallel.** Two UI siblings (iOS + web) consuming an existing API have no dep between them.

If you can't articulate a one-sentence reason for a dep, don't add one — parallel is safer than a wrong direction. Record the reasoning on the dependent child:

```bash
bd update <consumer-child-id> --append-notes="Depends on <producer-child-id> because: <one-sentence reason>."
```

### 4. Sync and return

```bash
bd dolt push
```

Report:

```
Autopilot: split <parent-id> into <child-1-id>, <child-2-id>, ... — loop will pick up ready children.
```

Return. No code work this tick. The next loop tick picks up the first unblocked child via `/beads:ready`; blocked children become ready once their producer closes.

## Phase 2: Dispatch

Create worktree:
```bash
bd worktree create ".claude/worktrees/autopilot-<bead-id>" --branch "autopilot/<bead-id>"
worktree_path="$(pwd)/.claude/worktrees/autopilot-<bead-id>"

# Workaround for beads GH#3421 (open in v1.0.2): bd worktree create does NOT
# propagate .beads/dolt-server.port, so bd commands inside the new worktree
# fail with "database not found on Dolt server at 127.0.0.1:<random>". Symlink
# the main tree's port file so the worker inherits the live server port.
ln -sf ../../../../.beads/dolt-server.port "${worktree_path}/.beads/dolt-server.port"
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

Clean up (pass the worktree NAME, not a path — bd rejects paths with "cannot remove main repository as worktree"):
```bash
bd worktree remove "autopilot-<bead-id>" --force
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

- **One bead per invocation.** Pick, ship, return. The loop handles repetition. Splitting counts as the invocation — after splitting a multi-worker bead, return without dispatching; the next tick picks up a ready child.
- **Split before you claim.** If a bead needs more than one worker type, split it into children before claiming. Never claim a multi-worker parent.
- **Never write code.** Workers handle implementation.
- **Never skip test verification.** Tests must pass after merge — this is the real evidence.
- **Never commit to main.** Session branch only.
- **One strike.** Failure -> block immediately, no retries.
- **Always `--append-notes`** (never `--notes`, which overwrites).
- **Always `bd dolt push`** after any bead state change.
- **Return fast when idle.**
- **Keep the cron alive while workers are in-flight.** Only cancel the cron when `bd ready` AND `bd list --status=in_progress` are BOTH empty. An empty ready queue with work still in-progress is a transient state — the loop should keep ticking until the session is genuinely idle.
