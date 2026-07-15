# 0042. A national, date-windowed backward backfill lane (Lane D)

Date: 2026-07-15

## Status

Accepted — additive to [0041](0041-poll-planit-on-a-churn-masked-delta-axis.md), ships disabled (`POLLING_BACKFILL_ENABLED=false`)

## Context

ADR 0041 replaced the per-authority drain with a churn-masked national delta poll (Lanes A/B/C), fixing the *flow* of new applications but explicitly deferring the *stock*: "Camden stays at 66/300 until a separate historical sweep runs... filling it is a separate exercise (a paced, worker-side historical sweep backwards on `start_date`) with its own budget and its own risks." This ADR is that separate exercise.

Two more gaps stack on top of the coverage hole:

1. **Field-level staleness.** GH#935 (PR #937, 2026-07-12) added seven columns (`reference`, `altid`, `associated_id`, `last_changed`, `last_scraped`, `scraper_name`, `other_fields`) that only populate on rows touched by a lane *after* that release. Every application ingested before it is missing those fields and stays missing forever unless something re-touches the row.
2. **A hard floor at "when we started polling".** Lanes A/B seed their watermark from PlanIt's current head specifically so a lane's first run is never a backfill (ADR 0041's rule 2, `c4f2a5ef`). Nothing in the current design ever looks further back than "now" for data no lane has fully swept.

Both are fixed by the same mechanism as the coverage-gap fix: a full-projection backward sweep. That is why this is one lane, not three.

**The constraint that overrides everything else in this design:** a resident must never get a push or email about something we only found by looking backward. Only Lane A (new applications) and Lane B (decisions) are allowed to notify. The backfill lane is a data-quality and coverage exercise, full stop.

### Why national and date-windowed, not per-authority

The first draft of this design scoped the sweep per-authority, mirroring Lane C's shape. That was wrong:

- **Lane C takes exactly one page, always** (`ReconciliationHandler.sweepAuthority`), fetching the top 300 most-recently-touched records per authority and stopping. Per-authority scoping there is a semantic requirement, not a cost optimisation: Lane C needs breadth *across* all 485 authorities every sweep, so a national top-300-by-`-last_different` query would be dominated by whichever authorities scraped most recently and would starve the rest.
- **Backfill's job is exhaustive, not a snapshot**, so it cannot get away with one page. But it does not need per-authority breadth either — it is not checking "is authority X's current state right", it is walking the whole national timeline once, and every authority's applications get touched as the window reaches their era regardless of whether the query names an authority.
- The failure ADR 0041 warns against — "an unbounded national sweep is the query we must never send" (`start_date >= today-365` -> `total: null` after 45s) — is about query *scope*, not pagination depth. That failure was a single request, one-sided, no upper bound. A **two-sided** bounded window (`start_date` *and* `end_date` both present) is the same shape ADR 0041 itself measured at 11.7s for a masked-but-unprefiltered national query.
- So: national, date-windowed, sliding backward — the same "one cursor, not 485" simplification ADR 0041 already proved out for Lanes A/B, inverted (shrinking the window's bound instead of growing a watermark) and applied to a full resweep instead of a delta.

## Decision

Add a fourth lane, `BackfillHandler` (`internal/polling/backfill.go`), that runs every poll cycle alongside Lanes A/B/C and spends a small, fixed page budget creeping backward through PlanIt's national history one date window at a time.

**State.** One singleton row (`backfill_state`, migration `0022_backfill_state.sql`) — not one row per authority: the fixed upper bound (`window_end`) of the date window currently being drained, a resumable record index within that window (`cursor_next_index`), a running count of records seen in the current window (`window_records_seen`), a count of consecutive fully-empty windows (`consecutive_empty_windows`), and a `complete` flag.

**Query shape.** Each cycle fetches up to `MaxPagesPerCycle` pages (default 2) of `FetchBackfillPage` (`internal/planit/backfill.go`): `start_date=<window_end - WindowWidthDays>&end_date=<window_end>`, `sort=-start_date`, `pg_sz=300`, the full `ingestSelectFields` projection (so the sweep can enrich every GH#935 field), `compress=on`, and — critically — **no `auth` param**. Every record returned is fed through the *existing, unmodified* `Ingester.Ingest`: identical records no-op, changed-or-NULL fields upsert (enrichment and gap-fill, for free, via the existing GH#935 three-bucket classification), first-seen records insert.

**Window sliding.** When a window is fully drained (`HasMorePages` goes false), the window slides back by `WindowWidthDays` and pagination resets to index 0. If a fully-drained window produced zero records, `consecutive_empty_windows` increments; after `EmptyWindowsBeforeComplete` (default 12, ~3 years of national silence) consecutive empty windows, the lane marks itself `complete` and every subsequent `Run` is a no-op — no fetch, forever.

**Crash safety.** State is persisted after every successfully-processed page, not batched to the end of a cycle. A crash mid-cycle loses nothing: re-fetching an already-ingested page is a free no-op under `Ingest`'s existing `HasSameBusinessFieldsAs`/`HasSameSilentFieldsAs` gates (the same reasoning ADR 0041 already relies on for Lanes A/B). A fetch or ingest error stops the loop for that cycle without persisting the failed page's progress — whatever prior pages in the same `Run` call succeeded stays persisted.

**The notification-safety guarantee is structural, not a runtime flag.** The backfill lane's `Ingester` is constructed with `NewIngester(apps, nil, nil)` — nil decision dispatcher, nil notification enqueuer. `BackfillHandler` has **no `WithFanOut` method at all**, unlike `NationalLaneHandler` and `ReconciliationHandler`. There is no call `cmd/worker`'s wiring could make, today or in a future edit, that attaches a notifier to this lane. A boolean or origin-tag that "suppresses" fan-out is one bug away from a resident getting a push about a 2019 application; omitting the method makes the unsafe wiring a compile error, not a code-review miss. `NationalPollHandler.Handle` runs Lane D unconditionally (nil-guarded) after Lane C, but never folds its counts into `ApplicationCount`/`AuthorityErrors`/termination reason — those describe the critical path, and Lane D is not on it.

**"Beyond when we first started polling" is discovered from the data, not a hardcoded floor date.** There is no reliable a-priori floor — different authorities' histories start at different points, and PlanIt's own digitisation depth is unknown — so the lane runs until enough consecutive windows come back empty.

**This is expected to be slow, and that is correct, not a shortfall.** Default `MaxPagesPerCycle=2` adds ~48 requests and ~14,400 records/day at an hourly cadence — comfortably inside the PlanIt red line (~1,500 requests/day) and a modest addition to ADR 0041's steady-state total (~140-150 requests/day, ~23,000 records/day). A full national sweep back through years of history is a multi-month background process by design. That trade — small budget, long horizon — is the point.

**Ships disabled** (`POLLING_BACKFILL_ENABLED=false`), mirroring the Lane C rollout precedent (`51eac0f0`, tc-5lu8h): brand-new polling code gets the dark-ship-then-soak treatment before flipping on.

## Consequences

### Positive

- Closes the coverage hole ADR 0041 deliberately deferred, using the exact mechanism (GH#935's three-bucket classification) that already exists — no new diffing logic.
- Every application ingested before GH#935 (2026-07-12) eventually gets its silent fields (`reference`, `altid`, `associated_id`, `scraper_name`, `other_fields`) populated as the sweep re-touches its era.
- Structurally cannot regress the one invariant that matters most: no backward-discovered application ever notifies a resident.
- Costs a small, fixed, indefinitely-sustainable slice of the PlanIt request budget — no growth cliff, no per-authority state to strand.

### Negative / risks

- **Slow by design.** Reaching the beginning of PlanIt's history is plausibly months away. Coverage metrics improve gradually, not immediately — anyone reading them must not mistake the pace for a bug.
- **No dashboard gauge yet** for how far back the window has reached, or a "% of estimated history covered" metric. Follow-up bead, not blocking this ship.
- **No reset mechanism for `complete`.** If the ingest field set widens again in future (another GH#935-style addition), there is currently no deliberate way to re-arm a completed sweep. Follow-up bead.
- **Unverified against the live API** (PlanIt is called from the deployed worker only, never a dev machine, per standing policy): that `sort=-start_date` behaves as every other PlanIt sort param does, and that a bounded-both-sides window stays in the ~11.7s territory ADR 0041 measured rather than degrading for an older, differently-shaped slice of history. Verified from telemetry after the first soak deploy, not before.

### Neutral

- `poll_state` is untouched — Lane D gets its own table (`backfill_state`) specifically to avoid colliding with `poll_state`'s planned column drop once the ADR 0041 soak completes.
- `internal/polling/handler.go` (the old, unwired per-authority drain) remains untouched and is not resurrected or built upon.
- `wirePollFanOut` (`cmd/worker/main.go`) is unchanged — there is nothing for it to wire onto Lane D.

## References

- ADR 0041 — the design this ADR fulfils the deferred half of ("filling it is a separate exercise... with its own budget and its own risks").
- GH#967 — the issue this ADR and its implementation (bead tc-8x9h3) satisfy.
- GH#935 / commit `1f450e2e` — the three-bucket ingest classification this design reuses unchanged as its enrichment/gap-fill mechanism.
- PR #964 / bead tc-5lu8h — the Lane C disable-behind-flag precedent this ADR's rollout posture mirrors.
- Commit `c4f2a5ef` / bead tc-5m3tw — the "seed from head, never backfill" fix for Lanes A/B, whose reasoning this design's own first-run window-fixing directly parallels.
