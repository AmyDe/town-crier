# 0044. A resumable, checkpointed national poll flow

Date: 2026-07-19

## Status

Proposed

Amends [0041](0041-poll-planit-on-a-churn-masked-delta-axis.md): it **keeps 0041's churn-masked delta axis** (what each lane queries is correct) and **replaces 0041's execution model** — the stateless, drain-the-whole-lane-per-cycle re-walk — with a resumable, per-page-checkpointed planner/executor loop. It also **replaces the per-authority Lane C** with a single national inverse-mask lane. Lane D ([0042](0042-historical-backward-backfill-lane.md)) folds into the same loop, gated to out-of-hours. [0043](0043-keep-lane-d-out-of-lane-a-mask-band.md) relies on the deleted per-authority Lane C mask-band sweep and **must be revisited** (see Consequences).

## Amendment (2026-09-06, issue #1127)

§5's "pinned epoch, contiguously tiled, self-healing on a stall" resumability model for **Lane C** was wrong. §5 walked Lane C as an ascending page-cursor over a pinned-upper-bound epoch and claimed that successive epochs tile contiguously on the change axis, so a storm stalling C mid-epoch just widens the next window with no gap and is therefore self-healing — that claim is false and backwards. `different_start=<epoch_lower>` is a coarse **floor only**; PlanIt has no `different_end` parameter, so the epoch ceiling is enforced client-side, but PlanIt still computes `total` and `sort` **server-side over every national record changed since that floor**. A wider window is a more expensive query, so a stall is positive feedback toward failure, not recovery.

This is the root cause of a ~7-week prod livelock (2026-07-19 to 2026-09-06, bead `tc-777e7`). Once the epoch cursor froze, the floor stayed ~7 weeks in the past; every in-hours cycle issued a query PlanIt could not answer inside its own 45s limit, returning HTTP 400 after 45s with `records_ingested = 0` and ~135s of hung requests per cycle against a free single-operator service. Part 1 (`tc-56ahl` / #1125) disabled Lane C in prod as mitigation, shipped `v0.21.27`.

**Replacement.** Lane C now queries a bounded **rolling `different=N` window**, with N recomputed each cycle as `clamp(days_since(last_clean_scan_at) + 1, 2, 3)`. The cap is a `const maxInverseMaskWindowDays = 3` — a fixed safety rule, not an operator dial, mirroring `nationalPageSize` (ADR 0041). The pinned-epoch anchor/seed and contiguous-tiling machinery are gone. `poll_state` row `-3` `HighWaterMark` is repurposed as `last_clean_scan_at`; `Cursor.DifferentStart` holds the scan's anchor date and is valid only while it equals today. The per-page `index=` checkpoint (GH#986) is retained. A scan that reaches the last page with no 429 and no hydration-cap bail is a **clean scan**: it stamps `last_clean_scan_at` and clears the cursor, which resets N to 2. On a calendar-day rollover mid-scan the cursor is discarded and the scan restarts at `index=0` with a freshly computed N. No schema migration.

**Accepted trade-off: a bounded coverage gap.** A Lane C outage longer than the cap (~3 days) leaves national records that changed while it was down, and are now older than the window, un-reconciled by Lane C until an explicit `poll_state` row `-3` reseed (Part 3, `tc-nkvil`). In the interim, new applications and decisions are still caught by Lanes A/B (24/7, freshness-gated), and the `records_seen` / `planit.total` invariant detects — without auto-recovering — a silent skip. Unbounded catch-up is exactly what livelocked; 0041's binding constraint, that cost is rows served and not requests sent, is unchanged.

**Lanes A/B are unaffected.** The §5 second bullet (descending, watermark-from-head) never used the pinned-epoch model and does not change.

**2026-09-06 live probes** (`end_date=2026-06-08`, `sort=last_different`, `pg_sz=300`, light select): `different_start=2026-07-19` (the frozen ~7-week epoch) and `different=50` both returned HTTP 400 at 45s; `different=1` gave total 3,203 at 0.10s, `different=2` gave 15,365 at 3.5s, `different=3` gave 24,848 at 3.3s, and `different=7` gave 84,167 at 43.1s — right at PlanIt's timeout cliff. `different=N` for N ≤ 3 is fast and inherently bounded; deep pages fetched via `index=` are ~0.18s.

## Amendment (2026-09-16, issue #1140)

**Withdrawn: the light projection and per-record hydration for Lane C.** Section 4's `inverseMaskSelectFields` (a 5-field light projection of `uid, area_id, app_state, decided_date, last_different`) paired with a per-record `id_match` straggler hydration (`FetchByUID`, one PlanIt request per differing row) is withdrawn. Lane C now fetches each page with the full `ingestSelectFields` projection verbatim, the same 25-field projection Lanes A, B and D already use, and it is never trimmed: issue #1027 is why. A narrower `select=` there nulled `other_fields` on upsert and destroyed roughly 47,000 rows' worth of data before it was caught, and `other_fields` is 54% of a record's bytes. When `inverseMaskDiffers` finds `app_state` or `decided_date` drifted (existence counts too), `processStraggler` now calls `Ingest` directly on the page row, with no second request. `inverseMaskDiffers` itself is unchanged and still gates on `app_state`/`decided_date` only, so section 4's anti-churn contract holds: `last_different` stays excluded from the diff, and Postgres writes stay proportional to real drift rather than to page count. `pg_sz` stays 300.

**Measured motivation.** Lane C ran in prod for about 30 hours under this old model (v0.21.29, 2026-09-07 10:09Z to 2026-09-08 14:58Z). It reached 4.5% of its 41,392-record window on 2026-09-07 and 4.9% on 2026-09-08, because every page ended on a hydration 429 after roughly 30 of the 300 rows it had already paid for, never on the page fetch itself. Lane C accounted for about 92% of the poller's PlanIt traffic across that window, and `last_clean_scan_at` stayed frozen at 2026-07-20 the whole time, because a scan that never reaches its last page never stamps it. Two roughly 3-hour polling blackouts followed inside 24 hours, and the run was rolled back in v0.21.31 / #1139 (bead `tc-vgbl7`).

**The batching probe that settled it.** ADR 0041 recorded batched `id_match` lookups as unproven. A live probe on 2026-09-08 disproved it: a comma-separated `id_match` list (three uids, light select) returned HTTP 200 with zero records. Hydration is irreducibly one PlanIt request per record, so no tuning of the old model could have closed this gap; the fan-out had to go.

**Accepted trade-off: more bytes for far fewer requests.** A complete scan of the measured window is 138 pages at `pg_sz=300`. The withdrawn model cost 138 page fetches plus roughly 12,400 hydration requests, about 12,540 total, for roughly 8.6 MB gzipped. The full-projection page model costs 138 requests for roughly 12.7 MB gzipped: 91 times fewer requests for 1.5 times the bytes. This runs deliberately against 0041's stated binding constraint that a free single-operator service's cost is rows served rather than requests sent. Requests are what PlanIt's rate limiter actually punishes, and the punitive multi-hour `Retry-After` values behind the tc-vgbl7 blackouts were a request-volume response, not a bytes one. It is also not a new shape: Lane D (ADR 0042) has run national pages at `pg_sz=300` with the identical `ingestSelectFields` projection in prod for months. Two 2026-09-08 probes confirm the widened shape is affordable: a first `different=3` page at `index=0` with the full projection took 12.159s and returned 300 records at 621,873 bytes uncompressed, 91,992 bytes gzipped (the server-side total+sort cost, paid once); the same query at `index=20000` took 1.293s for 685,765 bytes uncompressed, 91,898 bytes gzipped, confirming deep pages stay cheap under the wider projection too. Those probes also showed all 300 records at `index=0` carrying `last_different` on a single date, so `different=N` is date-granular within a UTC day, matching what section 5 already assumed.

**New safety rules replace the hydration cap.** With hydration gone, `maxHydrationsPerPass` no longer bounds anything and is deleted along with `hydrate`, `hydrationsThisPass` and `hydrationCapHit`. Three new rules take its place.

(a) *A per-cycle page cap.* `InverseMaskOptions.MaxPages`, loaded from `POLLING_LANE_C_MAX_PAGES_PER_CYCLE` (default 15), is enforced in `loadPlannerState` exactly as Lane A/B's own cap already is: once Lane C has run its cap for the cycle, the planner reports a synthetic `&LaneState{LastPollTime: now}` with no cursor, so it drops out of candidacy for the rest of the cycle without touching persisted state. Sizing: a worst-case `different=3` scan is 138 pages, the daytime window (07:00 to 19:00 Europe/London) gives about 12 hourly cycles, and 138 divided by 12 is 11.5 pages/cycle, so 15 leaves headroom for a lost cycle or two.

(b) *A section 6 cadence change.* `hasWorkC`'s 24-hour `laneCIdleAnchorInterval` is replaced by a UTC calendar-day anchor: a fresh scan is due once `LastPollTime`'s UTC date no longer matches today's, rather than 24 hours after it. The 24-hour anchor made each day's scan start at the previous day's finish time. A 138-page scan starting at 06:00Z at 15 pages/hour finishes around 15:00Z, so the next one could not start until 15:00Z the following day, got roughly 45 pages in before the 18:00Z window close, and was discarded whole by the existing UTC-midnight cursor reset: scans alternated between complete and discarded rather than completing daily. The calendar-day anchor starts each scan when the daytime window opens, which, sized against the 15-page cap above, finishes well inside it.

(c) *A recency gate on fan-out.* `InverseMaskLaneHandler.WithFanOut` now composes `recencyGatedDispatcher` and `recencyGatedEnqueuer` around the incoming collaborators inside the setter itself, the same structural pattern ADR 0047 uses for Lane E, so the wiring site can never hand Lane C an ungated notifier. The window is `InverseMaskOptions.NotifyRecencyWindow`, from `POLLING_LANE_C_NOTIFY_RECENCY_DAYS` (default 30). Because Lane C's band is `start_date` older than the 90-day mask by construction, this suppresses all `NewApplication` fan-out from Lane C outright, which is intended: an application filed more than 90 days ago is not new to anybody. A genuine recent `decided_date` on an old application still dispatches, which is the backstop role Lane C exists for.

**Ships dark.** `POLLING_LANE_C_ENABLED` stays `false` in prod. Enabling it is a separate, soak-gated infrastructure change, not part of this amendment. `tc-007fv` (the scheduler's 3-hour `RetryAfterCap`, which is what turned Lane C's hydration 429s into full polling blackouts) is a recommended prerequisite for that enable, but it is not fixed here.

## Amendment (2026-09-26, issue #1171)

Lane C has run in prod since v0.21.33 (2026-09-23, `tc-276by`) — stable, no blackouts, no failed spans — but it has never completed a scan. Every span still reports `poll.last_clean_scan_at=2026-07-20T15:48Z` and `poll.window_days=3`, unchanged since before the #1127 model replaced the pinned epoch. #1127/#1140 fixed the failure modes that stopped Lane C running at all; neither fixed a gap in the model itself — a scan's progress was **all-or-nothing**. `last_clean_scan_at` (`HighWaterMark`) advanced only when `!res.HasMorePages`, so a scan that ran for hours and covered most of its window, then hit the UTC-midnight cursor reset (§5's staleness guard, correct on its own terms — `different=N` is date-granular) before reaching its last page, discarded all of that progress: N stayed at the cap, the next day's scan was exactly as wide as the one that had just failed to finish, and the cycle repeated.

**Measured (prod, `AppDependencies`, 2026-09-24/25).** PlanIt allows roughly 15 requests/hour to Lane C across its two daytime cycles; each page advances the cursor 290 records (`nationalPageSize=300` minus the 10-record `laneCResumeOverlapRecords`), for throughput of roughly 3,800 records/hour. A `different=3` scan is roughly 34,350 records — about 9 uncontested hours — but Lane A/B's tier-1 planner priority (§3/§6, unchanged and correct) took every call for the first 3-4 daytime hours on both measured days, leaving Lane C 8-10 of its 12 daytime hours. On 2026-09-25 the cursor reached index 31,030 of 34,350 — 90% — when the daytime window closed at 17:18Z. Because the scan had not reached its last page, `last_clean_scan_at` never moved, N stayed 3, and 2026-09-26 started an identically-sized scan from scratch. 90% of a day's work was thrown away daily.

**Fix: a coverage watermark, reusing `PollCursor.WalkHead`.** Lane C's ascending walk visits rows in `last_different` order, so once the cursor has passed a row, every row in the window with an earlier `last_different` has already been checked — the information was on every page all along. `WalkHead` (GH#983, migration 0023's `cursor_walk_head` column), unused on Lane C's sentinel row, is repurposed as the scan's own in-flight coverage head: the maximum `last_different` over rows `processStraggler` has actually finished, updated after each row that succeeds (a hard error on row *i* counts only the rows before *i*; a zero `last_different`, never a genuine PlanIt value, never advances it). At a fresh scan's start, if the just-discarded stale cursor's `WalkHead` is more recent than the persisted `HighWaterMark`, it is folded into `HighWaterMark` before N is computed — so a scan that reached 90% coverage before the midnight reset resumes the next day computing N from that 90% mark, not from whatever `HighWaterMark` had been frozen at before.

**`HighWaterMark` moves ONLY at the two scan boundaries — never mid-scan.** N is recomputed from it on every page of a scan; if it moved mid-scan, N would change under an already-persisted `index=` offset, and that offset would then point at a different result set than the one it was computed against — silently skipping or re-reading rows. So `HighWaterMark` only ever changes at a fresh start (the fold above) or a clean completion (`now`, §5, unchanged); every mid-scan save, including the page-fetch 429/error path, persists it unchanged. The span attribute `poll.last_clean_scan_at` keeps its name (now the coverage mark in effect after the call) alongside a new `poll.coverage_head` for the in-flight progress.

**Expected effect.** 2026-09-25's partial scan would have folded its coverage mark forward at the next fresh start, so 2026-09-26 would run at N=2 — roughly two-thirds the size — fitting inside a daytime window even after a similar Lane A/B morning. A Lane A/B drain now only delays Lane C; it no longer costs it the day, closer to what #1140 §3(a)'s page-cap sizing assumed the all-or-nothing watermark had been quietly defeating.

**No schema migration, no new flag.** `cursor_walk_head` already persists and reads for every `poll_state` row; Lane C's row (`-3`) starts using a column it always had access to but never touched. Ships with no `POLLING_LANE_C_*` addition — the change only narrows N over time, which reduces PlanIt load — so rollback is a plain image redeploy back to a build that ignores `WalkHead` on this row again.

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

- **The planner is pure.** `NextWork` does no network and no writes: it reads typed `poll_state` plus the clock and returns the next unit of work as a **typed `WorkItem{Lane, PollCursor}`** — never a serialised URL. This is the non-flaky reading of "what to poll lives up a level": the executor constructs the PlanIt URL fresh from the typed lane config and cursor on every call, so mask cutoffs (`today − MASK_DAYS`) and Lane C's rolling `different=N` window are always recomputed, never stored and never stale.
- **The executor is dumb.** `NationalLaneHandler` becomes a one-page executor: build the query from lane config + cursor, fetch exactly one page, ingest, persist the advanced cursor, return. All existing query-building and ingest logic is reused; what moves is *where the page loop lives* (up into `Handle`, not inside each lane).
- **One process, one lease, one trigger, one budget.** The loop stays inside the single Service-Bus-triggered handler under one Postgres lease (ADR 0024): one content-free tick in, the derived work done, one `ComputeNextRun` → `PublishAt` out. A separate *planner job* is rejected (see below) — the separation is a code boundary, not a process boundary, precisely so the single-lease / no-dual-run / one-shared-`Retry-After`-budget invariants are preserved. This dissolves the `outC` backoff-fold gap outright: there is no per-lane fold to forget, only one `break` on the first 429.

### 2. Checkpoint after every page

The livelock fix. The executor persists a resume cursor **after each page**, reusing the machinery the per-authority Lane C already used: `PollCursor{DifferentStart, NextIndex, KnownTotal}`, where `NextIndex` is PlanIt's record-granular `index=` offset — immune to PlanIt's 1MB response-body truncation (GH#955 / tc-nlvpz). A 429 therefore costs **at most the one in-flight page**: the previous page's checkpoint is already committed, and the next session resumes at the next index in the same lane. Sessions that get only 1–2 calls still commit forward progress, so the backlog drains across sessions instead of livelocking.

### 3. Four lanes, LRU round-robin among whichever are eligible

| Lane | Purpose | Eligible | Walk | Freshness |
|---|---|---|---|---|
| A | new applications | 24/7 | descending, watermark-from-head | due every `POLLING_LANE_FRESHNESS_INTERVAL` (default 15m) or mid-drain |
| B | decisions | 24/7 | descending, watermark-from-head | due every 15m or mid-drain |
| C | inverse-mask reconciliation | 07:00–19:00 Europe/London | **ascending, page-cursor over a rolling `different=N` window** | mid-scan, or ≥ 24h since last scan |
| D | historical backfill (0042) | 19:00–07:00 Europe/London | its existing paced backward sweep | has backfill pages |

- **Round-robin is LRU over the eligible set, using existing state.** `poll_state` already carries `last_poll_time` per sentinel row and a `GetLeastRecentlyPolled` query. `NextWork` filters to lanes that are *eligible now* (A/B always; C only in the daytime window; D only out-of-hours) and *have work*, then picks the one with the oldest `last_poll_time`. That is round-robin with no new pointer column and no migration.
- **Freshness never pauses; heavy work is time-of-day split.** A (new apps) and B (decisions) stay live around the clock so a resident is notified the same day an application lands or is decided, honouring the product requirement. The heavy reconciliation band (C) runs only in the day; the multi-year backfill (D) owns the quiet overnight window, where it grinds between the occasional 15-minute A/B freshness probe. Windows and interval are config (`POLLING_DAY_START` / `POLLING_DAY_END`, default 07:00/19:00 Europe/London; `POLLING_LANE_FRESHNESS_INTERVAL`, default 15m).
- **The freshness interval prevents wasted probes.** Without it, a per-page A→B→C rotation would spend two-thirds of every call on near-empty A/B probes for the *hours* a Lane C backlog takes to drain — needless request pressure on a free service. Gating A/B to "due every 15m or mid-drain" means that during a C or D backlog, A/B jump the queue only every 15 minutes (still trivially same-day) and the backlog lane gets essentially every other call.

### 4. Lane C is the national inverse-mask query

Unchanged in intent from this ADR's superseded first draft — only its execution (now ascending + resumable, per §5) differs:

```
?different=<N>                                   # rolling window, N ∈ [2,3] days, recomputed per cycle (bounded → fast)
&end_date=<today − POLLING_LANE_A_MASK_DAYS>     # the INVERSE of Lane A's start_date mask: only the start_dates A/B exclude
&pg_sz=300                                       # PlanIt's documented default, unchanged
&select=uid,app_state,decided_date,last_different  # light projection ~100 bytes/record
&compress=on
```

Diff each light row against Postgres on **`app_state` and `decided_date`** (plus existence) and hydrate — a single-uid full fetch then the standard `Ingest` — only rows that are new or whose status actually changed. **Drop `last_different` from the diff:** it is bumped by every re-index, so keeping it flags every churned old record as a straggler (the observed hydration amplification), and only status changes matter for notification. Lane A masks `start_date ≥ today − MASK_DAYS`; Lane C masks the complement, so every recently-changed record is owned by exactly one lane.

### 5. Walk direction and cursor checkpointing — the resumability core

- **Lane C (backlog): ascending, page-cursor over a bounded rolling `different=N` window.** Each cycle the executor computes `N = clamp(days_since(last_clean_scan_at) + 1, 2, maxInverseMaskWindowDays)` (`maxInverseMaskWindowDays = 3`, a `const` safety rule not an operator dial, per 0041's `nationalPageSize`) and queries `different=N` — the national records changed in the last N days — walking them on `last_different` **ascending**, checkpointing `NextIndex` after every page. A scan that reaches the last page with no 429 and no hydration-cap bail is a **clean scan**: it stamps `last_clean_scan_at` and clears the cursor, so the next cycle recomputes a fresh N (steady state N = 2). A 429 or budget cut leaves the `index=` checkpoint and the next session resumes mid-scan; on a calendar-day rollover the window has shifted and `total` has moved, so the in-flight cursor is discarded and the scan restarts at `index=0`. Ascending is skip-safe: rows changing mid-scan sort in at the high end the walk reaches last, so an in-flight `index=` offset stays stable. The window is inherently bounded — `different=N` for N ≤ 3 is a fast, cheap query (~25k light rows at N = 3, ~3.3s) — so a stall cannot widen it past the cap. A steady-state scan serves the measured ~15,600 light rows/day.
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

- **The recent-start mask band loses its Lane C backstop.** The per-authority sweep incidentally re-checked recent-start applications — the coverage [ADR 0043](0043-keep-lane-d-out-of-lane-a-mask-band.md) (Proposed) relies on for its Decision 1. The inverse-mask Lane C deliberately does not touch the mask band. Recent-start misses now fall back to Lane A's own self-correcting watermark (which advances only on a clean page) and the `records_seen` / `planit.total` invariant (which *detects* a silent skip but does not auto-recover it). Measured exposure is currently zero and the acute cases were a one-time cutover-seed artefact (0043's evidence), so this is low-risk today, but it is a genuine reduction in defence-in-depth. **ADR 0043 must be revisited.**
- **`last_different`-only churn is not hydrated, by design.** A genuine field edit that changes neither `app_state` nor `decided_date` on an old application is not notified. That is the explicit product decision (only status changes matter), not a regression.
- **The planner is now load-bearing.** Lane eligibility and LRU ordering decide what gets polled. Mitigated because the planner is a pure function of typed state plus the clock, unit-tested exhaustively with no I/O.
- **A Lane C outage longer than the window cap leaves a bounded coverage gap.** There is no pinned epoch. If Lane C is down longer than the ~3-day `different=N` window cap, national records that changed while it was down and are now older than the window are not re-reconciled by Lane C until an explicit `poll_state` row `-3` reseed (Part 3, `tc-nkvil`). New applications and decisions in that gap are still caught by Lanes A/B (24/7, freshness-gated); the `records_seen` / `planit.total` invariant detects, but does not auto-recover, a silent skip. This is the deliberate cost of keeping catch-up bounded on a free single-operator service — unbounded catch-up is what livelocked Lane C (see Amendment).
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
