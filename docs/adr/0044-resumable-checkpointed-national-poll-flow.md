# 0044. A resumable, checkpointed national poll flow

Date: 2026-07-19

## Status

Proposed

Amends [0041](0041-poll-planit-on-a-churn-masked-delta-axis.md): it **keeps 0041's churn-masked delta axis** (what each lane queries is correct) and **replaces 0041's execution model** — the stateless, drain-the-whole-lane-per-cycle re-walk — with a resumable, per-page-checkpointed planner/executor loop. It also **replaces the per-authority Lane C** with a single national inverse-mask lane. Lane D ([0042](0042-historical-backward-backfill-lane.md)) folds into the same loop, gated to out-of-hours. [0043](0043-keep-lane-d-out-of-lane-a-mask-band.md) relies on the deleted per-authority Lane C mask-band sweep and **must be revisited** (see Consequences).

## Context

Two problems share one root.

**1. The per-authority Lane C has never worked in prod.** Measured against prod (Log Analytics + live PlanIt, 2026-07-17/18):

- Post-breaker, Lane C clears **~2 authorities per cycle** against the 50 it targets; the first full pass slipped from a ~10-hour goal to **over a week**.
- The per-authority driver issues ~186 PlanIt requests per cycle (485-authority projection pages plus straggler hydrations). When PlanIt rate-limits, the `tc-mc0hf` 429 circuit breaker trips on the first exhausted-retry 429 and halts the sweep. *Before* that breaker, Lane C ignored 429s and kept firing — **283 rate-limited responses in the 14:00 hour on 2026-07-17**, a direct red-line violation. *After* it, PlanIt 429s fell to ~1/hour but throughput collapsed.
- Lane C **discards the `Retry-After` value**, and `NationalPollHandler.Handle` omits Lane C's outcome (`outC`) from its rate-limit backoff fold, so a Lane C 429 reschedules nothing. The root cause is the per-authority **shape**: 485 sweeps plus hydration fan-out is a request volume that collides with a free service's rate limits.

**2. The stateless re-walk livelocks under PlanIt's escalating 429s.** 0041's national lanes advance their single watermark **only on a completely clean run** — any early stop (429, page-cap, context cut-off) leaves the watermark untouched and the next run re-walks the same range from PlanIt's head. That is a deliberate, safe failure direction *when a cycle almost always completes*. It is not safe under PlanIt's observed behaviour: 429s escalate the more the service is hit, so a session sometimes gets only **1–2 calls before a 429 with `Retry-After` 30–90s**. Each short session spends those calls re-treading the same head pages and never reaches the backlog — no forward progress is ever committed. 0041 removed the old per-authority drain's 485-cursor sprawl, but it removed the drain's *resumability* along with it.

**The binding constraint is unchanged (0041): a free single-operator service's cost is ROWS SERVED — rows retrieved and serialised — not requests sent.** Every choice below is made against that metric.

### The measurements that shape the fix

Live PlanIt and prod telemetry, 2026-07-17/18:

- **The inverse-mask slice is bounded and fast.** The band Lane C exists to cover is exactly the inverse of Lane A's mask — recently-changed records with a `start_date` older than the mask cutoff:

  ```
  GET /api/applics/json?different=1&end_date=2026-04-19&select=uid,app_state,decided_date&compress=on
    → total: 15,578 records, secs_taken 0.471
  ```

  No `total: null`, ~0.47s. Lanes A/B's masked band and this inverse band **partition** the national change axis with no gap or overlap, because both derive from the same `POLLING_LANE_A_MASK_DAYS`.
- `last_different` is PlanIt's own detection timestamp (data dictionary: *"when the source information on the planning authority website was last found to have changed"*), stamped at scrape time even for very old records — 2002-`start_date` applications observed carrying `last_different = today`. A change to an old application is therefore visible on the national change axis, which is what lets one national query see it.
- **A full-window re-scan is not an option.** Re-scanning `recent=30` (31,705) and `different=1` (31,286) every cycle serves **~250,000 rows/day**, ~10× the current design and essentially the ~224,000-rows/day shape 0041 already rejected. It is rejected here too.
- New-application inflow is ~1,057/day (`recent=30` = 31,705 over 30 days); decisions ~37,808 over 30 days. Both are small forward flows against the ~15,600/day inverse band.

## Decision

Restructure the poll cycle from a fixed `A → B → C → D` sequence of self-draining lanes into a **planner-driven, per-page-checkpointed executor loop**, keeping 0041's masked-delta axis.

### 1. Separate planning from execution, in one process

`NationalPollHandler.Handle` stops calling each lane's self-draining `Run`. Instead it runs a single loop over a **planner** and a **one-page executor**:

```
Handle(ctx):
    flusher.Reset()
    for {
        if budgetExhausted(ctx):    reason = TimeBounded; break   // work remains → resume soon
        item := planner.NextWork(state, clock)                    // {lane, cursor} | nil, pure
        if item == nil:             reason = Natural; break        // all lanes caught up → +1h
        out := execOnePage(ctx, item)                              // ONE fetch + ingest + checkpoint
        if out.rateLimited:         reason, retryAfter = RateLimited, out.retryAfter; break
        if out.err:                 log(out.err); break            // safe stop; last checkpoint holds
    }
    flusher.Flush(ctx)
    return PollPlanItResult{ reason, retryAfter, ... }
```

- **The planner is pure.** `NextWork` does no network and no writes: it reads typed `poll_state` plus the clock and returns the next unit of work as a **typed `WorkItem{Lane, PollCursor}`** — never a serialised URL. This is the non-flaky reading of "what to poll lives up a level": the executor constructs the PlanIt URL fresh from the typed lane config and cursor on every call, so mask cutoffs (`today − MASK_DAYS`) and epoch bounds are always recomputed, never stored and never stale.
- **The executor is dumb.** `NationalLaneHandler` becomes a one-page executor: build the query from lane config + cursor, fetch exactly one page, ingest, persist the advanced cursor, return. All existing query-building and ingest logic is reused; what moves is *where the page loop lives* (up into `Handle`, not inside each lane).
- **One process, one lease, one trigger, one budget.** The loop stays inside the single Service-Bus-triggered handler under one Postgres lease (ADR 0024): one content-free tick in, the derived work done, one `ComputeNextRun` → `PublishAt` out. A separate *planner job* is rejected (see below) — the separation is a code boundary, not a process boundary, precisely so the single-lease / no-dual-run / one-shared-`Retry-After`-budget invariants are preserved. This dissolves the `outC` backoff-fold gap outright: there is no per-lane fold to forget, only one `break` on the first 429.

### 2. Checkpoint after every page

The livelock fix. The executor persists a resume cursor **after each page**, reusing the machinery the per-authority Lane C already used: `PollCursor{DifferentStart, NextIndex, KnownTotal}`, where `NextIndex` is PlanIt's record-granular `index=` offset — immune to PlanIt's 1MB response-body truncation (GH#955 / tc-nlvpz). A 429 therefore costs **at most the one in-flight page**: the previous page's checkpoint is already committed, and the next session resumes at the next index in the same lane. Sessions that get only 1–2 calls still commit forward progress, so the backlog drains across sessions instead of livelocking.

### 3. Four lanes, LRU round-robin among whichever are eligible

| Lane | Purpose | Eligible | Walk | Freshness |
|---|---|---|---|---|
| A | new applications | 24/7 | descending, watermark-from-head | due every `POLLING_LANE_FRESHNESS_INTERVAL` (default 15m) or mid-drain |
| B | decisions | 24/7 | descending, watermark-from-head | due every 15m or mid-drain |
| C | inverse-mask reconciliation | 07:00–19:00 Europe/London | **ascending, page-cursor over a pinned epoch** | has unwalked epoch pages |
| D | historical backfill (0042) | 19:00–07:00 Europe/London | its existing paced backward sweep | has backfill pages |

- **Round-robin is LRU over the eligible set, using existing state.** `poll_state` already carries `last_poll_time` per sentinel row and a `GetLeastRecentlyPolled` query. `NextWork` filters to lanes that are *eligible now* (A/B always; C only in the daytime window; D only out-of-hours) and *have work*, then picks the one with the oldest `last_poll_time`. That is round-robin with no new pointer column and no migration.
- **Freshness never pauses; heavy work is time-of-day split.** A (new apps) and B (decisions) stay live around the clock so a resident is notified the same day an application lands or is decided, honouring the product requirement. The heavy reconciliation band (C) runs only in the day; the multi-year backfill (D) owns the quiet overnight window, where it grinds between the occasional 15-minute A/B freshness probe. Windows and interval are config (`POLLING_DAY_START` / `POLLING_DAY_END`, default 07:00/19:00 Europe/London; `POLLING_LANE_FRESHNESS_INTERVAL`, default 15m).
- **The freshness interval prevents wasted probes.** Without it, a per-page A→B→C rotation would spend two-thirds of every call on near-empty A/B probes for the *hours* a Lane C backlog takes to drain — needless request pressure on a free service. Gating A/B to "due every 15m or mid-drain" means that during a C or D backlog, A/B jump the queue only every 15 minutes (still trivially same-day) and the backlog lane gets essentially every other call.

### 4. Lane C is the national inverse-mask query

Unchanged in intent from this ADR's superseded first draft — only its execution (now ascending + resumable, per §5) differs:

```
?different_start=<epoch lower bound>             # coarse date-granular prefilter (bounded → fast)
&end_date=<today − POLLING_LANE_A_MASK_DAYS>     # the INVERSE of Lane A's start_date mask: only the start_dates A/B exclude
&pg_sz=300                                       # PlanIt's documented default, unchanged
&select=uid,app_state,decided_date,last_different  # light projection ~100 bytes/record
&compress=on
```

Diff each light row against Postgres on **`app_state` and `decided_date`** (plus existence) and hydrate — a single-uid full fetch then the standard `Ingest` — only rows that are new or whose status actually changed. **Drop `last_different` from the diff:** it is bumped by every re-index, so keeping it flags every churned old record as a straggler (the observed hydration amplification), and only status changes matter for notification. Lane A masks `start_date ≥ today − MASK_DAYS`; Lane C masks the complement, so every recently-changed record is owned by exactly one lane.

### 5. Walk direction and epoch tiling — the resumability core

- **Lane C (backlog): ascending, page-cursor over a pinned-upper-bound epoch.** At epoch start the executor captures a fixed upper bound `epoch_upper = now`; it walks the window `[epoch_lower, epoch_upper]` on `last_different` **ascending**, checkpointing `NextIndex` after every page. When the epoch drains, it advances `epoch_lower = epoch_upper` and clears the cursor; the next anchor sets a fresh `epoch_upper = now`. Successive epochs **tile contiguously** on the change axis, so a storm that stalls C mid-epoch simply makes the next window wider — no gap, self-healing. Ascending is skip-safe: nothing can enter a fixed-upper-bound window, and records that leave it (re-changed, so `last_different` jumps past `epoch_upper`) leave at the high end the walk reaches last. A daily epoch serves the measured ~15,600 light rows/day.
- **Lanes A/B (freshness): descending, watermark-from-head, also checkpointed.** These stay newest-first because freshness is their job. They keep 0041's timestamp watermark, advancing it only on a clean drain to the boundary; the per-page cursor is insurance for the rare multi-page walk (e.g. an overnight A/B accumulation), and the existing `PollCursor` staleness guard — "valid only while the high-water-mark date still matches" — auto-invalidates it when a fresh head arrives, so a new change is never skipped. In steady state each is ~1 light page, so re-walking on a 429 is cheap.

### 6. Cadence: perma-run while there is work, from the existing scheduler unchanged

`NextRunScheduler.ComputeNextRun` already has exactly the three states the loop needs — **no scheduler change**:

- planner returns `nil` (all lanes caught up) → **Natural, +1h**.
- handler budget (`POLLING_HANDLER_BUDGET_SECONDS`, default 240s, inside the 5-minute Service Bus lock cap) hit with backlog remaining → **TimeBounded, +1m** — the "continue soon" resume.
- 429 on any page → **RateLimited, `Retry-After` capped at 3h**, honoured uniformly for every lane.

So while a C or D backlog exists, each session ends TimeBounded and re-triggers in ~1 minute, grinding continuously; under a storm it ends RateLimited and resumes after the (short) `Retry-After`; when everything is caught up it settles to the hourly natural rhythm. The near-continuous poller is an emergent property of checkpointing plus the existing scheduler, not a new mechanism.

### 7. Delete

The per-authority sweep (`ReconciliationHandler`), its `Due`-interval gating, the `AuthoritiesPerCycle` / `MaxStragglersPerAuthority` / `LookbackDays` options, and the `tc-mc0hf` 429 circuit breaker. All are moot under the national inverse-mask lane and the single-loop 429 handling.

## Consequences

### Positive

- **The livelock is fixed.** Per-page checkpointing means a session that gets only 1–2 calls still commits forward progress; the backlog drains across sessions instead of re-treading PlanIt's head forever.
- **Old-application status changes are caught on the axis where they live**, as one bounded national query (0.47s), not 485 per-authority sweeps. The 429 collisions, the breaker, the 2-authorities-per-cycle crawl, the resumable-authority cursor and the `outC` backoff-fold gap all delete — a net reduction in code and operational surface.
- **Freshness never pauses.** New applications and decisions are polled around the clock, so same-day notification holds regardless of the time-of-day split; the heavy reconciliation and the multi-year backfill are scheduled where they do not compete with it.
- **Resumability is regained without the 485-cursor sprawl.** 3–4 lane cursors replace 485 authority cursors — the old drain's forward-progress guarantee with 0041's no-per-authority win.
- **Politer to PlanIt.** One 429 stops the session cold and it resumes mid-backlog, rather than re-reading the head. `Retry-After` is honoured on every lane through the single shared scheduler.

### Negative / risks

- **The recent-start mask band loses its Lane C backstop.** The per-authority sweep incidentally re-checked recent-start applications — the coverage [ADR 0043](0043-keep-lane-d-out-of-lane-a-mask-band.md) (Proposed) relies on for its Decision 1. The inverse-mask Lane C deliberately does not touch the mask band. Recent-start misses now fall back to Lane A's own self-healing (its watermark advances only on a clean page) and the `records_seen` / `planit.total` invariant (which *detects* a silent skip but does not auto-recover it). Measured exposure is currently zero and the acute cases were a one-time cutover-seed artefact (0043's evidence), so this is low-risk today, but it is a genuine reduction in defence-in-depth. **ADR 0043 must be revisited.**
- **`last_different`-only churn is not hydrated, by design.** A genuine field edit that changes neither `app_state` nor `decided_date` on an old application is not notified. That is the explicit product decision (only status changes matter), not a regression.
- **The planner is now load-bearing.** Lane eligibility and LRU ordering decide what gets polled. Mitigated because the planner is a pure function of typed state plus the clock, unit-tested exhaustively with no I/O.
- **An epoch's pinned upper bound can defer a mid-epoch re-change.** A record that re-changes after the epoch anchor leaves the window and is caught by the next epoch or by Lane A — an acceptable one-epoch delay for a reconciliation-class change, since Lane A owns freshness.
- **Each lane concentrates on one national request path**, like 0041's A/B: a single failing query stalls that lane for a cycle. Mitigated by the same retry/backoff discipline and by the queries being cheap and bounded.

### Neutral

- ADR 0024 (Service-Bus-only triggering), the Ingester's fan-out, and the `applications` table are unchanged.
- **No schema migration.** `poll_state` already holds `cursor_different_start`, `cursor_next_index`, `cursor_known_total`, `high_water_mark` and `last_poll_time` (migrations 0003, 0021). The sentinel rows `-1` (A) / `-2` (B) / `-3` (C) are reused; Lane D keeps its own backfill state. Rollback stays a pure image redeploy.

### Explicitly rejected

- **Full-window re-scan of `recent=30` + `different=1` every cycle.** ~250,000 rows/day — a rows-served red-line violation and essentially the ~224,000-rows/day shape 0041 already rejected. The inverse-mask delta reaches the same coverage at ~1/16th the rows.
- **A separate Service Bus trigger chain for Lane C.** All lanes must stay serial under one lease and one shared `Retry-After` budget (ADR 0024); a second chain double-polls PlanIt.
- **Keeping Lane C per-authority and only fixing its pacing.** The 485-request volume is inherent to the shape; pacing it cannot make it cheap. Nationalising the query removes the volume.
- **The descending stateless re-walk (0041's execution model).** It livelocks under PlanIt's escalating 429s — the core problem this ADR fixes. Superseded, not merely tuned.
- **A literal "store the URL to call in a DB row" executor.** Too flaky (stored URLs rot as `today` moves). The planner persists *typed* work (lane + cursor); the executor builds the URL fresh each call.
- **A separate planner *job*.** Reintroduces the cross-job coordination, second trigger and lease complexity this design exists to avoid. The planner/executor split is a code boundary within the one handler.

### Deferred (not rejected)

- **An active mask-band backstop.** A once-daily light `recent=30` existence scan (~32,000 light rows/day, ~3 MB) would restore an active recovery path for recent-start misses (the 0043 dependency). Out of scope here to keep the change surgical; file as follow-up if the `records_seen` / `planit.total` invariant ever fires or exposure appears.

## Sequencing

Lands as one change: delete the per-authority Lane C, ship the planner/executor loop with all four lanes, and re-enable Lane C in its national inverse-mask form (it is currently disabled in prod via the `tc-tuge8` / GH#971 churn, so there is no per-authority sweep to keep running alongside). The ADR 0043 Decision 1 revisit is independent and can follow.

## References

- [ADR 0041](0041-poll-planit-on-a-churn-masked-delta-axis.md) — the churn-masked national delta axis (kept) and the per-authority Lane C and stateless re-walk execution (replaced).
- [ADR 0042](0042-historical-backward-backfill-lane.md) — Lane D, the paced historical backfill, folded into this loop and gated out-of-hours.
- [ADR 0043](0043-keep-lane-d-out-of-lane-a-mask-band.md) (Proposed) — relies on the per-authority Lane C mask-band sweep; must be revisited.
- `tc-mc0hf` — the Lane C 429 circuit breaker this deletes.
- `tc-tuge8` / GH#971 — the Lane C 400-query repair and enable/disable churn.
- GH#955 / `tc-nlvpz` — the record-granular `cursor_next_index`, immune to PlanIt's 1MB truncation, reused here as the per-page checkpoint.
- Bead `tc-9i7sa` — this design.
- PlanIt data dictionary — `last_different`: https://www.planit.org.uk/dictionary/
- Measurements taken against live PlanIt and prod telemetry (Log Analytics) on 2026-07-17/18.
