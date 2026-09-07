# 0047. A looping recent-window sweep over the recent start_date band (Lane E)

Date: 2026-09-07

## Status

Proposed

Amends [0044](0044-resumable-checkpointed-national-poll-flow.md): it keeps 0044's planner/executor loop, its per-page checkpointing and its tiered LRU selection unchanged, and adds a fifth lane to the model. It also depends on [0042](0042-historical-backward-backfill-lane.md)'s Lane D remaining exactly as it is, for reasons set out below.

## Context

### Nothing verifies the recent band

Lane A queries `start_date >= today - POLLING_LANE_A_MASK_DAYS` (90 days) and walks descending on `last_different` from a single global watermark. On a completed walk that watermark jumps to `walkHead`, the first record of page 0 (`nationallane.go:449`), and it only ever moves forward. Any application whose `last_different` sits below the watermark by the time we could have seen it is gone. There is no second look.

Lane C cannot cover those. Its `end_date` ceiling is the exact inverse of Lane A's mask, so by construction it only ever examines applications with a `start_date` **older** than `today - 90d`. Lane D creeps backward through history and is years from the present. So an application filed inside the last 90 days that Lane A skipped is checked by nothing, ever, and no metric shows it.

That gap is structural, not a bug in any one lane. Every existing lane walks a delta axis or a receding historical window. None of them re-reads the band Lane A is responsible for.

The gap is also not the tc-777e7 gap. Lanes A/B were confirmed healthy throughout that investigation, and tc-777e7's 19 July to 5 September hole is in the old-`start_date` band that Lane C owns. Lane E does not close it and must not be described as doing so.

### Why not just widen Lane C

Lane C's reach is capped at `different=3` by `maxInverseMaskWindowDays`, and 0044's amendment explains why: PlanIt has no `different_end` parameter, so a wider window is a strictly more expensive server-side query, and a stall becomes positive feedback toward failure. 0044 measured `different=7` at 43.1 seconds against PlanIt's 45 second limit. Widening N recreates the livelock. The `different=N` axis cannot be the backstop.

### What the measurements say about the alternative

Live PlanIt probes on 2026-09-07, six calls at 75 second intervals, `sort=-start_date`, `pg_sz=300`, full ingest projection, `compress=on`:

| Window | `total` | `secs_taken` |
|---|---|---|
| 7d (2026-08-31 to 09-07) | 2,914 | 0.158 |
| 15d (2026-08-23 to 09-07) | 9,824 | 1.42 |
| 30d (2026-08-08 to 09-07) | 28,599 | 2.839 |
| 45d (2026-07-24 to 09-07) | 52,410 | 6.813 |
| 90d (2026-06-09 to 09-07) | 131,788 | 17.166 |
| 30d at `index=3000` | 28,599 | 0.225 |

Three things follow.

**A two-sided `start_date` window dodges the cliff that `different=N` cannot.** Bounding both ends gives PlanIt a finite set to sort, which is the same property 0042 relied on for Lane D.

**All the cost is the first page of a window.** Page 10 of a 30 day window costs 0.225s against 2.839s for page 0. PlanIt pays for `total` and the sort once, then serves pages nearly free. So narrow windows are cheaper end to end, not merely safer: a 90 day lap tiled as six 15 day windows spends about 8.5s on window setup, tiled as one 90 day window it spends 17.2s, and the per-page cost after setup is identical. The intuition that a wider window saves work is backwards.

**The recent band is 131,788 records, about 440 pages.** That is the size of one full verification lap, and it sets the pacing arithmetic.

A full-projection page is roughly 544KB for 300 records, about 1.8KB each. Sweeping 440 of those per lap serialises around 240MB. 0041's binding constraint is rows served rather than requests sent, and the row count is the same whatever projection is used, but there is no reason to serialise fifteen times more bytes than the job needs.

## Decision

Add a fifth lane, `RecentSweepHandler` (`internal/polling/recentsweep.go`), that loops backward through the recent `start_date` band in bounded windows, verifies each PlanIt row against Postgres, and hydrates only what diverges.

**Axis: bounded `start_date` windows, not `different=N`.** Lane E tiles `[lap_anchor - DepthDays, lap_anchor]` into `WindowWidthDays` slices and walks them newest first, sliding the upper bound back as each slice drains. `DepthDays` defaults to `POLLING_LANE_A_MASK_DAYS`, so Lanes A, E and C partition the national `start_date` axis with no gap and no overlap, and stay partitioned if the mask is ever retuned. `WindowWidthDays` defaults to 15, hard-capped at 30 by a package constant rather than an operator dial, mirroring `nationalPageSize` and `maxInverseMaskWindowDays`.

**It never completes.** When the cursor crosses the floor the lap ends: the lap counter increments, the anchor is re-cut at today, and the walk starts again from the top. There is no `Complete` flag. Lane D has one because history runs out; the recent band never does.

**The lap anchor is fixed for the lap.** A floor recomputed from a moving `today` recedes as the lap runs, so the lap would chase it and never land. Anchoring makes each lap a fixed 90 day band with a predictable duration, and re-anchoring at lap end means the days that elapsed during the lap are picked up by the next one.

**Light projection, hydrate on divergence.** Lane E sweeps with `{uid, area_id, app_state, decided_date, start_date}` and hydrates a full record via the existing `FetchByUID` only when the row is missing from Postgres or when `inverseMaskDiffers` reports a change. `start_date` is in the light set because it is the sort field and PlanIt returns 400 if the sort field is absent from `select`. This is Lane C's proven shape, reused rather than reinvented, and it cuts the serialised bytes by roughly fifteen times on a happy path that should be almost entirely no-ops. Hydration is bounded by `maxHydrationsPerSweepTurn = 10`; hitting the cap checkpoints and returns cleanly rather than erroring, exactly as Lane C does, so a real backlog drains across laps instead of bursting into PlanIt's 429 threshold.

**It notifies, behind an event-specific recency gate.** This is the sharpest difference from Lane D, and the reason Lane E is a new type rather than a second `BackfillHandler`. 0042 made "Lane D can never notify" a compile-time guarantee by hardwiring `NewIngester(apps, nil, nil)` and omitting `WithFanOut` entirely. That guarantee is worth more than the duplication saved by parameterising it, so `BackfillHandler` is untouched and Lane E is its own type.

Lane E's gate is composed on top of the existing collaborators rather than built into them. Two decorators over `polling.NotificationEnqueuer` and `polling.DecisionDispatcher` drop the call when the relevant date is outside `POLLING_LANE_E_NOTIFY_RECENCY_DAYS` (default 30). The test is event-specific: `start_date` gates a new-application fan-out, `decided_date` gates a decision. `last_different` is rejected as the test because a PlanIt re-index bumps it, and 0044 records 2002-dated applications carrying `last_different = today`, so it would wave through anything. Both decorators fail closed on a nil date.

The gate covers **both** paths, not just decisions. `Ingester.Ingest` fans out to the enqueuer on any notifiable change including a first-time insert, which is exactly what a backstop lane produces. The only existing back-dating guard, `zone.CreatedAt.After(app.LastDifferent)` in `enqueuer.go:145`, does not help: a zone created a year ago passes it for anything Lane E finds.

`RecentSweepHandler.WithFanOut` applies the decorators itself and then rebuilds its `Ingester`. The wiring site cannot hand Lane E an ungated notifier. That is the same structural instinct 0042 used when it omitted `WithFanOut` from Lane D, applied to a lane that does need to notify: make the unsafe wiring unreachable rather than relying on review.

**Its own state table.** `recent_sweep_state`, with an explicit `id smallint PRIMARY KEY CHECK (id = 1)` and a `WHERE id = 1` on save. `backfill_state` is keyless and its `UPDATE` carries no `WHERE`, so a second row there would make every Lane E save silently clobber Lane D's cursor.

**Scheduling: Lane D's out-of-hours slot, paced by an idle interval.** Lane E is eligible exactly when Lane D is, and 0044's existing tier-2 LRU round-robins them. `laneEIdleInterval = 1h` caps it at one turn per hour, which is what makes the pacing predictable: at the 1 hour natural cadence that is one turn per out-of-hours cycle, regardless of how often a `TimeBounded` or `RateLimited` termination re-fires the cycle. With `MaxPagesPerCycle = 6` that is 72 pages a day, so a 440 page lap lands in about six days at full health and closer to seven once cycles lost to tier-1 contention, 429s and timeouts are counted. Lane E stays in tier 2 and never outranks Lanes A or B.

**Ships dark** behind `POLLING_LANE_E_ENABLED=false`, following the Lane C and Lane D precedent. It matters more here than it did for Lane D, because this lane can send a push.

**Its own stall alert.** Lane E is deliberately excluded from `alert-planit-lane-stuck-shared`, whose condition is `records_ingested == 0 and total > 0`. That is Lane E working correctly: a backstop re-sweeping data that is already right ingests nothing while `planit.total` reports the whole window. Feeding it to that rule would produce a permanently firing alert and teach everyone to ignore it. Lane E's forward-progress signal is the cursor instead: a new rule watches for `lane_e.window_end` failing to change across two days of spans.

## Consequences

### Positive

- The recent `start_date` band gets an exhaustive verifier for the first time. An application Lane A missed is found within roughly a lap, and a missing row is inserted and, if recent enough, notified.
- The failure mode is now visible. A wedged Lane E raises a cursor-stall alert rather than looking identical to a healthy one, which is exactly how tc-777e7 went unnoticed for seven weeks.
- Lane D's compile-time no-notification guarantee survives intact. Nothing in this ADR weakens 0042.
- The notification gate produces no record at all for a stale event, rather than a stored row with a suppressed push. Data minimisation by construction.
- The design is measured rather than assumed. Window width, lap size and pacing all come from live probes taken on the day of the decision, and the surprising result (narrow windows are cheaper, not just safer) is baked in.

### Negative / risks

- **It costs about 21,600 rows a day**, a real addition to the steady state. For scale, 0044 rejected a shape serving ~250,000 rows/day and 0041 rejected one at ~224,000, so this sits well inside what both treat as sustainable. `POLLING_LANE_E_MAX_PAGES_PER_CYCLE` is the dial to turn down first if PlanIt shows strain.
- **Lane D gets less of the out-of-hours slot.** Two lanes now share it via LRU, so the historical backfill roughly halves in pace. 0042 already calls that sweep a multi-month background process; it becomes a longer one.
- **Pagination drift within a window.** A record newly scraped into a window mid-sweep shifts `index=` offsets and can be stepped over. The lap structure is the mitigation: a record skipped this lap is seen next lap. Lane D carries the same exposure and lives with it.
- **Hydration amplification on a first lap.** If there is a genuine backlog, early laps will hit the hydration cap every turn. That is bounded and self-limiting by design, but it means the first lap tells you very little about steady-state cost. Do not tune the pacing dial from it.
- **No silent-field enrichment.** The light projection cannot see `other_fields`, `reference`, `altid`, `associated_id` or `scraper_name`, so unlike Lane D this lane does nothing for GH#935 staleness. Deliberate: enrichment is Lane D's job and paying 1.8KB a record to duplicate it would cost fifteen times the bytes for a benefit that is already covered.
- **Recency is a product judgement, not a technical one.** 30 days bounds how stale a notification can be, but a resident pushed about a 29 day old application with the consultation window closed is still being told something of limited use. `notifydispatch/record.go` stamps every record `CreatedAt: now`, so it will present as fresh. Revisit the number after the soak.
- **Unverified against live PlanIt in the lane's own shape.** The probes used the full projection, so the light-projection timings are inferred to be no worse rather than measured. Confirm from telemetry after the dark soak, not before.

### Neutral

- No change to `Ingester`, `Enqueuer` or `DecisionDispatcher` internals. The gate is composition, not modification.
- 0044's planner stays pure. Lane E adds an eligibility case and a `hasWork` predicate, both functions of typed state and the clock.
- `poll_state` and `backfill_state` are untouched. Lane E gets its own table for the same reason Lane D got one.
- The `inverseMaskDiffers` name stays Lane C flavoured despite serving both lanes. A shared name would be tidier; a duplicate test would be worse.

## References

- ADR 0041 - the churn-masked delta axis, and the rows-served constraint every choice here is made against.
- ADR 0042 - Lane D, whose window-walk this lane is modelled on and whose no-fan-out guarantee it deliberately leaves alone.
- ADR 0044 - the planner/executor loop this lane joins, the Lane C rolling-window amendment, and the `different=7` at 43.1s measurement that rules out widening Lane C instead.
- GH#1134 - the implementation issue for this decision.
- GH#1130 / PR#1132 - the zero-forward-progress alert this lane extends and, for the stuck rule, deliberately opts out of.
- tc-777e7 - the Lane C livelock. Related context, and explicitly not fixed by this lane.
- tc-3f8hh - the design bead.
