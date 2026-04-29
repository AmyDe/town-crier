---
name: autopilot-loop
description: End-to-end wrapper that drains the entire ready/in-progress bead backlog via the autopilot skill, then ships the resulting branch as a PR, watches it through the gate, and either cuts a patch release on success or attempts ONE recovery cycle on failure. MUST use this skill whenever the user says "autopilot loop", "autopilot-loop", "drain and ship", "work the backlog and release", "ship the backlog", or "/autopilot-loop". Replaces the manual `/loop 5m /autopilot ... then ship then release` pattern with a single command.
---

# Autopilot Loop

End-to-end orchestrator: drain → ship → release, with one recovery attempt on PR failure.

```
DRAIN ──▶ SHIP ──▶ ┬─ merged ──▶ RELEASE (patch) ──▶ DONE
                   │
                   └─ failed ──▶ RECOVER (once) ──▶ DRAIN ──▶ SHIP ──▶ ┬─ merged ──▶ RELEASE ──▶ DONE
                                                                       └─ failed ──▶ STOP (report)
```

This skill is invoked once and runs the full sequence inline. It does not use a cron — it calls `/autopilot`, `/ship`, and `/release` directly via the Skill tool, one phase at a time, until the workflow terminates.

## State

The orchestrator tracks ONE bit of state in-memory: `recovery_attempted` (bool, starts `false`). Set it to `true` the first time you enter RECOVER. Never enter RECOVER twice — if ship fails after recovery, stop.

## Phase 1: DRAIN

Goal: process every ready bead and any beads already in-progress, until the backlog is empty.

### 1.1 Pre-flight

Confirm we're on `main` or an `autopilot/*` branch. If on anything else, stop and report.

```bash
git fetch origin
git rev-parse --abbrev-ref HEAD
```

If on `main`, proceed — `/autopilot` will create the session branch on its first tick.

### 1.2 Drain loop

Repeat the following until BOTH `bd ready` AND `bd list --status=in_progress` return empty:

```bash
ready_count=$(bd ready --json 2>/dev/null | jq 'length' 2>/dev/null || echo 0)
inprog_count=$(bd list --status=in_progress --json 2>/dev/null | jq 'length' 2>/dev/null || echo 0)
echo "DRAIN: ready=$ready_count in_progress=$inprog_count"
```

While `ready_count > 0` OR `inprog_count > 0`:

1. **If `ready_count > 0`**: invoke the `autopilot` skill via the Skill tool. It picks one bead, dispatches a worker, validates, merges, and returns. One bead per invocation.
2. **If `ready_count == 0` but `inprog_count > 0`**: a worker is still finishing. Wait briefly with Monitor:
   ```bash
   until [ "$(bd list --status=in_progress --json 2>/dev/null | jq 'length')" = "0" ]; do sleep 30; done
   ```
   Wrap this in Monitor so you're notified when the in-progress queue clears. Once cleared, recheck `ready_count` (a closed bead may have unblocked successors).
3. **Re-check both counts** after each autopilot invocation. The list shrinks as beads close, but new ones can appear if a worker splits a multi-worker parent.

**Safety bound:** Cap drain at 50 autopilot invocations per drain pass. If you hit the cap, stop and report — something is wrong (probably the same bead being repeatedly blocked).

### 1.3 Anything to ship?

When the drain loop exits, check whether autopilot actually produced commits:

```bash
current_branch=$(git rev-parse --abbrev-ref HEAD)
if [ "$current_branch" = "main" ]; then
  echo "No autopilot session branch — nothing to ship."
  exit 0  # report and stop
fi
git log main..HEAD --oneline
```

- **No commits ahead of main:** autopilot found no work or every bead failed. Stop and report.
- **Commits exist:** proceed to SHIP.

## Phase 2: SHIP

Invoke the `ship` skill via the Skill tool. It will:

- Push the current `autopilot/*` branch (renaming it to `auto/*` if needed)
- Create a PR
- Wait for the PR Gate via `gh pr checks --watch`
- Squash-merge on success (auto-merge is wired up in CI)
- Reset main and clean up

### 2.1 Capture the PR outcome

`/ship` reports the PR URL and final state. Parse the result:

- **Merged:** proceed to Phase 3 (RELEASE).
- **Gate failed:** proceed to Phase 4 (RECOVER) if `recovery_attempted == false`, otherwise STOP.
- **Timeout** (gate never completed within ship's window): poll for resolution before deciding:
  ```bash
  pr_number=<pr-from-ship>
  until state=$(gh pr view "$pr_number" --json state -q '.state'); [ "$state" = "MERGED" ] || [ "$state" = "CLOSED" ] || gh pr checks "$pr_number" --json bucket -q '.[].bucket' | grep -q '^fail$'; do sleep 60; done
  ```
  Wrap in Monitor. When it exits, evaluate: merged → RELEASE; failed → RECOVER or STOP; closed without merge → STOP (manual intervention).

## Phase 3: RELEASE

Invoke the `release` skill via the Skill tool with intent "patch release". The release skill handles version computation, release notes, and `gh release create`.

When release completes, report the release URL and exit cleanly. **DONE.**

## Phase 4: RECOVER (one attempt only)

Set `recovery_attempted = true`. If it was already `true`, skip this phase and STOP.

### 4.1 Diagnose the failures

```bash
pr_number=<the-failed-pr>
gh pr checks "$pr_number" --json name,state,bucket
```

For each check with `bucket == "fail"`, fetch the failed run logs:

```bash
gh pr checks "$pr_number" --json name,link | jq -r '.[] | select(.link != null) | .link'
# For each failed check link, extract the run ID and:
gh run view <run-id> --log-failed
```

### 4.2 File recovery beads

For each distinct failure, file a bead. Reference the failed PR in the description so the worker has full context:

```bash
bd create \
  --title="Fix CI failure: <short description>" \
  --description="PR #<pr_number> gate failure. Check: <name>. Cause: <one-line summary>. Log excerpt: <relevant lines>. See: <pr-url>" \
  --type=bug \
  --priority=1
```

If the failure is environmental (staging deploy, integration tests failing on infra) rather than code, file ONE bead summarising the issue with priority=1 and STOP — don't enter another drain loop, since environmental fixes need human attention. Report and exit.

If failures are unfixable (e.g. unclear cause, no actionable signal in logs), STOP and report.

### 4.3 Reset to a clean session base

Before re-draining, get back to a state autopilot can work from. After a `/ship` failure, the session is still on the failed `auto/*` branch with the PR open.

```bash
# Close the failed PR with a pointer to the recovery effort
gh pr close <pr_number> --comment "Autopilot-loop: closing — recovery beads filed, will reopen as a new PR after fixes."

# Get back to a clean main
git checkout main
git fetch origin
git reset --hard origin/main
git remote prune origin
```

This forces autopilot's Phase 0 (session-branch setup) to start fresh on the next DRAIN.

### 4.4 Loop back

```bash
bd dolt push
```

Return to **Phase 1: DRAIN**. The new beads should be ready. Process them, re-ship, then evaluate the PR outcome:

- **Merged:** Phase 3 (RELEASE) → DONE.
- **Failed again:** STOP and report. Both PRs and the original failures should be in the report.

## Phase 5: STOP / DONE

Always end with a single summary message containing:

- What was achieved (beads closed, PR URL, release URL if any)
- Why we stopped (success / retry exhausted / unfixable failure)
- Any follow-up beads filed

Update the orchestrator bead's notes via `bd update tc-ophh --append-notes="..."` if relevant.

## Rules

- **One recovery attempt only.** After RECOVER → SHIP fails, stop.
- **Never call `/release` if the PR didn't merge.** A failed gate means main hasn't moved.
- **Never bypass `/ship`.** All merging goes through PR + gate, never `git push origin main`.
- **Never deploy directly.** If a failure points at "deploy this manually", file a bead and stop.
- **Always invoke sub-skills via the Skill tool**, not by re-implementing their logic. The sub-skills own their own edge cases.
- **Cap the drain at 50 autopilot ticks per pass.** Infinite drains usually mean a single bead is being repeatedly blocked.
- **Default to patch release.** If commits since the last tag look like a feature release (any `feat:`), the release skill will detect that itself — don't override.
- **Bead-first.** This skill assumes there is at least one ready bead at start. If there are none and nothing in-progress, report "Nothing to do" and exit.

## Edge cases

- **Already on a PR branch with an open PR:** DRAIN may not produce new commits. Skip to SHIP — it will detect the existing PR and watch it.
- **Existing autopilot cron from a previous session:** if `CronList` shows one, leave it alone; the cron will run autopilot ticks in parallel. The orchestrator's drain loop is idempotent — a bead claimed by the cron will simply not appear in `bd ready` next iteration.
- **`bd ready` lists a bead that autopilot keeps marking blocked:** the drain cap (50) will catch this. Report the offending bead in the stop message.
- **Recovery beads themselves fail:** by design we stop after one recovery cycle. Report all PRs and failures.
- **Release tag already exists at HEAD:** the release skill handles this; if it asks "skip, force, or pick different version", default to "skip" and report DONE.
