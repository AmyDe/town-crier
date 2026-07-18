# 0044. Replace per-authority Lane C with a national inverse-mask reconciliation

Date: 2026-07-18

## Status

Proposed

Amends the Lane C design in [0041](0041-poll-planit-on-a-churn-masked-delta-axis.md). Lanes A, B and D are unchanged. Interacts with [0043](0043-keep-lane-d-out-of-lane-a-mask-band.md) — see Consequences.

## Context

ADR 0041 defined Lane C as the completeness backstop for what the churn-masked A/B delta structurally cannot see: decisions with no `decided_date`, rows with no `app_state`, applications whose `start_date` falls outside both masks, and upstream corrections. It implemented it **per authority** — 485 light-projection sweeps, run weekly, diffed against Postgres, hydrating only drifted rows.

**In production it has never worked as intended.** Measured against prod (Log Analytics + live PlanIt, 2026-07-17/18):

- Post-breaker, Lane C clears **~2 authorities per cycle** against the 50 it targets. The first full pass slipped from a ~10-hour goal to **over a week**.
- The per-authority driver issues ~186 PlanIt requests per cycle (per-authority projection pages plus straggler hydrations). When PlanIt rate-limits, the `tc-mc0hf` 429 circuit breaker trips on the first exhausted-retry 429 and halts the sweep. *Before* that breaker, Lane C ignored 429s and kept firing — **283 rate-limited responses in the 14:00 hour on 2026-07-17**, a direct red-line violation. *After* it, PlanIt 429s fell to ~1/hour but throughput collapsed to 2 authorities/cycle.
- Lane C **discards the `Retry-After` value** and never folds its rate-limit into the cycle cadence, so a Lane C 429 parks nothing: the next hourly cycle simply resumes ~2 authorities further along. The crawl is structural, not a tuning knob.

**The root cause is the per-authority shape.** 485 sweeps plus hydration fan-out is a request volume that collides with a free service's rate limits, and the resumable-authority cursor and the bespoke breaker are brittle scaffolding around that volume.

ADR 0041 chose per-authority "because an unbounded national sweep is the query we must never send (`start_date >= today-365` returned `total: null` after **45 seconds**)." That reasoning holds for an *unbounded* query. It does not hold for a national query bounded by `different_start` — the same shape Lanes A/B already send, which PlanIt answers in ~0.2s.

### The measurements that unlock the fix

Live PlanIt, 2026-07-17/18:

- The slice Lane C exists to cover is exactly the **inverse of Lane A's mask**: recently-changed records with a `start_date` older than the mask cutoff. As one national query:

  ```
  GET /api/applics/json?different=1&end_date=2026-04-19&select=uid,app_state,decided_date&compress=on
    → total: 15,578 records, secs_taken 0.471
  ```

  Bounded, fast, no `total: null`. Lanes A/B's masked band and this inverse band **partition** the national change axis.

- `last_different` is PlanIt's own detection timestamp — data dictionary: *"when the source information on the planning authority website was last found to have changed"* — stamped at scrape time even for very old records. Observed at the head of the `-last_different` stream: 2002-`start_date` applications carrying `last_different = today`. A change to an old application is therefore visible on the national change axis, which is what makes the inverse-mask query able to see it.

## Decision

Replace Lane C's per-authority sweep with a single **national inverse-mask delta lane** — the mirror image of Lane A — run in the poll cycle:

```
?different_start=<Lane C watermark>              # coarse date-granular prefilter (bounded → fast)
&end_date=<today - POLLING_LANE_A_MASK_DAYS>     # the INVERSE of Lane A's start_date mask: only the start_dates A/B exclude
&sort=-last_different                            # newest-changed first
&pg_sz=300                                       # PlanIt's documented default, unchanged
&select=uid,app_state,decided_date,last_different  # light projection, unchanged from today's Lane C
&compress=on
```

- Where Lane A masks `start_date >= today - POLLING_LANE_A_MASK_DAYS` (recent applications), Lane C masks `end_date = today - POLLING_LANE_A_MASK_DAYS` (`start_date <=`, old applications). Both bands derive from the **same** config value, so they cannot drift apart or overlap: every recently-changed record is owned by exactly one lane.
- **Walk descending to one global Lane C watermark**, exactly as Lanes A/B do. Reuse the existing `poll_state` sentinel `-3` row, now holding a watermark timestamp instead of an authority-list cursor. The watermark advances only on a clean walk. This is a *delta*, not a re-scan: each cycle fetches only what changed since the watermark.
- Diff each light row against Postgres on **`app_state` and `decided_date`** (plus existence), and hydrate — a single-uid full fetch, then the standard `Ingest` — only rows that are new or whose status actually changed. **Drop `last_different` from the diff:** it is bumped by every re-index, so keeping it flags every churned old record as a straggler (the observed hydration amplification), and only status changes matter for notification.
- A 429 on this national query folds into the cycle's rate-limit backoff exactly as A/B/D do (`ComputeNextRun` → `Retry-After` → `PublishAt` scheduled enqueue). **Delete the bespoke 429 breaker.**
- **Lanes run in series under one trigger, sharing one budget — Lane C is not a separate chain.** A → B → C → D run sequentially in a single `Handle` call, on the single throttled PlanIt client (one global inter-request delay), under one lease, driven by one content-free Service Bus tick (ADR 0024). There is exactly **one** `Retry-After` decision per cycle: all lanes' 429s fold into a single `ComputeNextRun`, which schedules the single next trigger. Lane C therefore cannot consume a rate-limit budget that A/B/D compete for — the budget *is* the single serialized cycle. This also repairs a live gap: today's `Handle` omits Lane C's outcome from the backoff fold, so a Lane C 429 currently reschedules nothing; under this ADR `outC` must be included (and `reconciliationOutcome` must carry the parsed `Retry-After`, which it discards today).
- **Delete:** the per-authority sweep, the resumable authority-list cursor, the `Due`/interval gating, and the `AuthoritiesPerCycle` / `MaxStragglersPerAuthority` / `LookbackDays` options along with the `tc-mc0hf` circuit breaker.
- Lanes A, B and D are unchanged.

### Request budget

| Lane | Cadence | Requests/day | Records/day |
|---|---|---|---|
| A — new applications | hourly, national | ~24–30 | ~1,700 |
| B — decisions | hourly, national | ~24–30 | ~1,700 |
| C — inverse-mask reconciliation | hourly, national (delta) | ~50 | ~15,600 light (re-index day; far fewer when quiet) |
| Hydration of Lane C status drift | as needed | ~20 | small |

Walked as a watermark delta (not a re-scan), the inverse band is ~50 light pages/day at ~100 bytes/record — in line with 0041's ~19,000-rows/day Lane C budget, and nowhere near the **~250,000 rows/day** a full-window `recent=30` + `different=1` re-scan would have cost. Because Retry-After is honoured through the shared scheduler, Lane C can neither hammer PlanIt nor starve behind its own breaker.

## Consequences

### Positive

- Lane C stops being 485 per-authority requests and becomes ~50 national pages. The 429 collisions, the breaker, the 2-authorities-per-cycle crawl, the `Due` interval and the resumable-authority cursor all delete. This is a net reduction in code and operational surface.
- **Old-application status changes are caught on the axis where they live.** A withdrawal, or a decision PlanIt records without a `decided_date`, on an application whose `start_date` is outside Lane A's mask, bumps `last_different` and carries an old `start_date` — so it lands in this query and is hydrated within the cycle. This is the coverage the per-authority sweep was meant to provide and never reliably did.
- Retry-After is honoured uniformly across every lane; Lane C can no longer saturate the rate-limit budget independently of A/B.
- The mask bands are complementary and config-derived, so Lane A (recent) and Lane C (old) partition the change axis with no gap and no overlap.

### Negative / risks

- **The recent-start mask band loses its Lane C backstop.** The per-authority sweep incidentally re-checked recent-start applications too — the coverage [ADR 0043](0043-keep-lane-d-out-of-lane-a-mask-band.md) (Proposed) relies on for its Decision 1 (*"that band is exactly what Lane C sweeps weekly, and Lane C both hydrates and notifies"*). The inverse-mask Lane C deliberately does not touch the mask band. Recent-start misses now fall back to **Lane A's own self-healing** (its watermark advances only on a clean page, so a transient error re-reads next cycle) and the **`records_seen` / `planit.total` invariant** (which *detects* a silent skip but does not auto-recover it). Measured exposure is currently zero and the acute cases were a one-time cutover-seed artefact (0043's evidence), so this is low-risk today — but it is a genuine reduction in defence-in-depth. **ADR 0043 must be revisited:** its Decision 1 justification no longer holds as written.
- **Deferred, not rejected — an active mask-band backstop.** A once-daily light `recent=30` existence scan (~32,000 light rows/day, ~3 MB) would restore an active recovery path for recent-start misses. It is out of scope here to keep this change surgical; file it as follow-up if the invariant ever fires or exposure appears.
- **`last_different`-only churn is no longer hydrated, by design.** A genuine field edit that changes neither `app_state` nor `decided_date` on an old application is not caught. That is the explicit product decision (only status changes matter for notification), not a regression against a working system.
- **A national query concentrates Lane C onto one request path**, like Lanes A/B: a single failing query stops the whole lane for a cycle. Mitigated by the same retry/backoff discipline and by the query being cheap (0.47s) and bounded.

### Neutral

- ADR 0024 (Service-Bus-only triggering) and the Ingester's fan-out are unchanged.
- The `applications` table is unchanged. `poll_state` keeps the sentinel `-3` row, now a watermark rather than a cursor.

### Explicitly rejected — convert Lanes A/B to full-window re-scans

The first shape considered was to re-scan `recent=30` and `different=1` every cycle so the design self-heals by brute force. It fails 0041's central, measured principle — that a free service's cost is **rows retrieved, not requests sent**. Re-scanning both windows serves **~250,000 rows/day**, ~10× the current design and essentially the ~224,000-rows/day design 0041 already rejected. The inverse-mask query reaches the same coverage as a delta at ~1/16th the rows.

### Explicitly rejected — keep Lane C per-authority and only fix its pacing

Honouring Retry-After per authority (the old Service-Bus-per-message model) would stop the hammering, but it cannot make a 485-request sweep cheap: the request volume is inherent to the shape. Nationalising the query removes the volume rather than pacing it.

## Sequencing

Lane C is currently disabled in prod (the `tc-tuge8` / GH#971 enable-disable churn). This ADR replaces its handler outright, so it lands as a single change with Lane C re-enabled in its new form — there is no per-authority sweep to keep running alongside. ADR 0043's Decision 1 revisit is independent and can follow.

## References

- [ADR 0041](0041-poll-planit-on-a-churn-masked-delta-axis.md) — the churn-masked national delta and the per-authority Lane C this amends.
- [ADR 0043](0043-keep-lane-d-out-of-lane-a-mask-band.md) (Proposed) — relies on Lane C's mask-band sweep; must be revisited in light of this.
- `tc-mc0hf` — the Lane C 429 circuit breaker this deletes.
- `tc-tuge8` / GH#971 — the Lane C 400-query repair and the enable/disable churn.
- Bead `tc-9i7sa` — this work.
- PlanIt data dictionary — `last_different` = *"when the source information ... was last found to have changed"*: https://www.planit.org.uk/dictionary/
- Measurements taken against live PlanIt and prod telemetry (Log Analytics) on 2026-07-17/18.
