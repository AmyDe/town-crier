---
name: autopilot-loop
description: End-to-end wrapper that drains the entire ready/in-progress bead backlog via the autopilot skill, then ships the resulting branch as a PR, watches it through the gate, and either cuts a patch release on success or attempts ONE recovery cycle on failure. On exit, arms an hourly cron so beads created later (e.g. from autonomous triage while the user is away) get processed automatically. MUST use this skill whenever the user says "autopilot loop", "autopilot-loop", "drain and ship", "work the backlog and release", "ship the backlog", or "/autopilot-loop". Replaces the manual `/loop 5m /autopilot ... then ship then release` pattern with a single command.
---

# Autopilot Loop

End-to-end orchestrator: drain → ship → release, with one recovery attempt on PR failure. Self-perpetuating: arms an hourly cron at exit so new beads get picked up automatically.

```
[entry] ──▶ §0 cron-tick fast-path ──▶ (idle?) ──▶ ENSURE NEXT TICK ──▶ DONE
                       │
                       └─ (work present) ──▶ DRAIN ──▶ SHIP ──▶ ┬─ merged ──▶ RELEASE ──▶ ENSURE NEXT TICK ──▶ DONE
                                                                │
                                                                └─ failed ──▶ RECOVER (once) ──▶ DRAIN ──▶ SHIP ──▶ ┬─ merged ──▶ RELEASE ──▶ ENSURE NEXT TICK ──▶ DONE
                                                                                                                    └─ failed ──▶ ENSURE NEXT TICK ──▶ STOP (report)
```

This skill is invoked once and runs the full sequence inline. It calls `/autopilot`, `/ship`, and `/release` directly via the Skill tool, one phase at a time. Before exiting, it uses `CronCreate` to arm an hourly tick that re-invokes itself, so the orchestrator keeps draining new beads without manual restart. The hourly cron is registered idempotently (via `CronList` first) so re-entry from a tick does not stack duplicate jobs.

## Phase 0: Cron-tick fast-path

This phase runs on every entry, including the very first interactive invocation. Its job is to make tick-fired re-entries cheap when there is nothing to do.

```bash
ready_count=$(bd ready --json 2>/dev/null | jq 'length' 2>/dev/null || echo 0)
inprog_count=$(bd list --status=in_progress --json 2>/dev/null | jq 'length' 2>/dev/null || echo 0)
echo "Pre-flight: ready=$ready_count in_progress=$inprog_count"
```

- **Both zero:** there is no work. Jump to **Phase 6: ENSURE NEXT TICK**, then exit. Do NOT run drain/ship/release. Final summary should be one line ("autopilot-loop: idle, cron armed for next hour"), not a full report.
- **Either non-zero:** proceed to Phase 1 (DRAIN).

This is what makes the hourly cron safe — an idle tick costs one `bd ready` + one `CronList`, then exits.

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

### 1.3 Authorized stop conditions

The drain may exit ONLY when one of these conditions fires. Anything else is unauthorized — keep draining.

| ID | Condition | Action |
|----|-----------|--------|
| **A** | `bd ready` is empty AND `bd list --status=in_progress` is empty | Proceed to §1.5 (Anything to ship?) |
| **B** | 50-tick cap reached (autopilot has been invoked 50 times this drain pass) | STOP and report — likely a stuck bead |
| **C** | The same bead was marked blocked in two consecutive ticks | STOP and report — that bead is the offender |
| **D** | An autopilot or ship sub-skill reported an error that requires Phase 4 (RECOVER) | Branch to RECOVER per the existing flow |
| **E** | The user has explicitly aborted (via signal or instruction) | STOP and report state-of-play |

Track the tick count and the last-blocked-bead-id in-memory across iterations to evaluate B and C. The pre-flight in §5.1 will verify which condition fired before allowing termination.

### 1.4 Forbidden stop reasons

These are the patterns the orchestrator's own judgment will rationalize. Recognize them and ignore them — they are NOT authorized stops:

- **"Context budget feels heavy" / "tokens are climbing"** — orchestrator context is not a stop condition. Long drains are the whole point of this skill.
- **"The next bead looks complex" / "this could time out"** — the worker, not the orchestrator, decides what's tractable. Dispatch and let one-strike-block handle it.
- **"We have enough to ship already"** — there is no "enough." The drain ships what's drained, not what feels sufficient.
- **"Session is getting long"** — duration is not a stop condition.
- **"I'm worried this will fail"** — fear is not a stop condition. Dispatch and observe.
- **"It's safer to checkpoint now"** — checkpointing mid-drain is not a sanctioned operation. Drain to one of A–E.

If you find yourself reaching for any of these reasons, stop, re-read §1.3, and continue the drain.

### 1.5 Anything to ship?

When the drain loop exits, check whether autopilot actually produced commits:

```bash
current_branch=$(git rev-parse --abbrev-ref HEAD)
if [ "$current_branch" = "main" ]; then
  echo "No autopilot session branch — nothing to ship."
  exit 0  # report and stop
fi
git log main..HEAD --oneline
```

- **No commits ahead of main:** autopilot found no work or every bead failed. Run **Phase 6: ENSURE NEXT TICK**, then stop and report.
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

When release completes, run **Phase 6: ENSURE NEXT TICK**, then report the release URL and exit cleanly. **DONE.**

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

If failures are unfixable (e.g. unclear cause, no actionable signal in logs), run **Phase 6: ENSURE NEXT TICK** then STOP and report.

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

- **Merged:** Phase 3 (RELEASE) → ENSURE NEXT TICK → DONE.
- **Failed again:** Run **Phase 6: ENSURE NEXT TICK** then STOP and report. Both PRs and the original failures should be in the report.

## Phase 5: STOP / DONE

### 5.1 Mandatory pre-flight: are we authorized to stop?

Before producing any final summary, run this check:

```bash
ready_count=$(bd ready --json 2>/dev/null | jq 'length' 2>/dev/null || echo 0)
inprog_count=$(bd list --status=in_progress --json 2>/dev/null | jq 'length' 2>/dev/null || echo 0)
```

If `ready_count > 0` OR `inprog_count > 0`, the orchestrator MUST have triggered one of the authorized stop conditions B, C, D, or E from §1.3. Verify which one fired and record it explicitly:

- **Condition B** (50-tick cap): tick counter must be ≥ 50.
- **Condition C** (same bead blocked twice): last-blocked-bead-id this tick must equal last-blocked-bead-id previous tick.
- **Condition D** (RECOVER exhausted): `recovery_attempted == true` AND the second ship attempt failed.
- **Condition E** (user abort): explicit user signal in the conversation transcript.

Additionally, condition A is satisfied trivially when `ready_count == 0` AND `inprog_count == 0` — DRAIN reached its natural end. Successful Phase 3 (RELEASE) completion implies A.

If none of A/B/C/D/E applies and `ready_count > 0`, this is an **UNAUTHORIZED STOP**. Report exactly:

```
STOP UNAUTHORIZED — drain has <N> ready beads but no §1.3 stop condition fired.
Returning to Phase 1.2 to continue draining.
```

…and re-enter the drain loop. Do NOT produce a summary message; do NOT exit. Re-read §1.4 if you can't articulate why you tried to stop — the reason is almost certainly forbidden.

### 5.2 Final summary

Once the pre-flight confirms an authorized stop, run **Phase 6: ENSURE NEXT TICK** UNLESS the stop reason is Condition E (user abort) — in that case the user explicitly told us to stop, so do not re-arm.

Then produce a single summary message containing:

- What was achieved (beads closed, PR URL, release URL if any)
- Why we stopped — name the condition (A/B/C/D/E) explicitly
- Any follow-up beads filed
- **Cron status** — one line stating whether the hourly cron was armed (and the job ID), or skipped because of user abort, with a `CronDelete` reminder when armed.

Update the orchestrator bead's notes via `bd update tc-ophh --append-notes="..."` if relevant.

## Phase 6: ENSURE NEXT TICK

Goal: guarantee that an hourly `/autopilot-loop` cron is registered before exit, so beads created later (e.g. by an autonomous triage process) get drained and shipped without manual restart.

This phase is **idempotent** and **never asks the user for input** — it must run cleanly when the user is away from the keyboard.

### 6.1 Skip conditions

Skip this phase only when:

- **Condition E (user abort)** fired in §1.3 — the user explicitly told us to stop. Re-arming would defy that instruction.

In every other exit path (DONE after RELEASE, DONE after recovery success, STOP after recovery exhausted, STOP after no-commits, STOP after 50-tick cap, STOP after same-bead-blocked-twice, idle exit from §0), proceed.

### 6.2 Check for an existing cron

```
CronList
```

Inspect the returned jobs. If any job's `prompt` contains `/autopilot-loop` (or otherwise re-invokes this skill), it is already armed. Do nothing — return.

### 6.3 Register the hourly cron

If no existing autopilot-loop cron is present:

```
CronCreate
  cron: "7 * * * *"
  prompt: "/autopilot-loop"
  recurring: true
  durable: true
```

Notes on the parameter choices:

- `cron: "7 * * * *"` — hourly at :07 to avoid the :00 fleet-wide thundering herd (see CronCreate guidance).
- `recurring: true` — fires every hour, not one-shot.
- `durable: true` — persisted to `.claude/scheduled_tasks.json` so the cron survives Claude Code restarts. The user is walking away from the machine; in-memory cron would die with the session.
- 7-day auto-expiry still applies. The first tick after expiry that finds work will re-register a fresh cron via this same Phase 6, so the loop is self-healing as long as it fires at least once per week.

Capture the returned job ID and surface it in the §5.2 final summary.

### 6.4 Failure handling

If `CronCreate` fails (tool error, permission, etc.), do NOT retry, do NOT block exit, do NOT escalate to the user. Log the failure in the final summary so the next interactive session can see it, and exit normally — a missed hourly schedule is recoverable; blocking exit is not.

## Rules

- **One recovery attempt only.** After RECOVER → SHIP fails, stop.
- **Never call `/release` if the PR didn't merge.** A failed gate means main hasn't moved.
- **Never bypass `/ship`.** All merging goes through PR + gate, never `git push origin main`.
- **Never deploy directly.** If a failure points at "deploy this manually", file a bead and stop.
- **Always invoke sub-skills via the Skill tool**, not by re-implementing their logic. The sub-skills own their own edge cases.
- **Cap the drain at 50 autopilot ticks per pass.** Infinite drains usually mean a single bead is being repeatedly blocked.
- **Do not invent stop conditions.** §1.3 enumerates every authorized reason to leave the drain (A–E). §1.4 lists the reasons that masquerade as stop conditions but aren't. §5.1 mechanically rejects unauthorized stops and re-enters the drain.
- **Default to patch release.** If commits since the last tag look like a feature release (any `feat:`), the release skill will detect that itself — don't override.
- **Bead-first.** This skill no longer requires a ready bead at start — Phase 0 short-circuits the empty case to "ensure cron, exit" so hourly ticks are cheap.
- **Always re-arm the hourly cron at exit (Phase 6) — except on user abort.** The skill is self-perpetuating by design so beads created later (autonomous triage, deferred decisions) get drained without manual restart.
- **Never ask the user before scheduling the cron.** The whole point of Phase 6 is that the user can walk away from the machine. Any prompt-to-confirm defeats the design.

## Edge cases

- **Already on a PR branch with an open PR:** DRAIN may not produce new commits. Skip to SHIP — it will detect the existing PR and watch it.
- **Existing autopilot cron from a previous session:** if `CronList` shows one, leave it alone; the cron will run autopilot ticks in parallel. The orchestrator's drain loop is idempotent — a bead claimed by the cron will simply not appear in `bd ready` next iteration.
- **`bd ready` lists a bead that autopilot keeps marking blocked:** the drain cap (50) will catch this. Report the offending bead in the stop message.
- **Recovery beads themselves fail:** by design we stop after one recovery cycle. Report all PRs and failures.
- **Release tag already exists at HEAD:** the release skill handles this; if it asks "skip, force, or pick different version", default to "skip" and report DONE.
