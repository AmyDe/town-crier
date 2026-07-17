# 0043. Keep Lane D out of Lane A's mask band, and record ingest provenance

Date: 2026-07-17

## Status

Proposed

Additive to [0041](0041-poll-planit-on-a-churn-masked-delta-axis.md) and [0042](0042-historical-backward-backfill-lane.md). Decision 2 reverses [0041](0041-poll-planit-on-a-churn-masked-delta-axis.md)'s "no schema migration" constraint (GH #962 constraint 4) and needs an explicit accept before it ships.

## Context

ADR 0041 and ADR 0042 are each correct in isolation. Together they leave a hole neither one anticipated.

**ADR 0041 makes Lane C the backstop against a silent skip.** Its own words (line 195): *"the risk is a silent skip, not a crash... Lane C's coverage metric is the mitigation, and it is the reason Lane C ships in the same release rather than later."* Lane C detects a skip by fetching each authority's top 300 by `last_different` and flagging **uids PlanIt has that Postgres does not**. That comparison is its entire detection mechanism.

**ADR 0042 makes Lane D structurally unable to notify.** `BackfillHandler` has no `WithFanOut` method at all, so no wiring can attach a notifier to it. That guarantee is sound and it is the right one: nobody should get a push about a 2019 application because we swept backward past it.

**The interaction:** once Lane D inserts a record Lane A skipped, the uid is present in Postgres, so Lane C's missing-uid test no longer fires. **Lane D erases the evidence Lane C needs.** The backstop goes blind, silently, and the record is never notified — not late, never. ADR 0042 reasoned carefully about Lane D not notifying; it did not consider Lane D destroying Lane C's ability to detect that something else should have.

### Evidence

Four Camden applications (`start_date` 2026-07-09 to 07-14) sit in Postgres inside a Pro user's watch zone that predates them, and were never notified. Their `last_different` is 2026-07-15 03:26:53-03:29:15 — below Lane A's cutover seed watermark of 05:16:14, and therefore invisible to Lane A's descending walk, which stops at the boundary. Lane D swept that `start_date` band in its first hours after flip-on (2026-07-15 21:25).

Sixteen paid-tier in-zone applications over 60 days were never notified in total, but the set is mixed: seven North Tyneside (2026-07-13) and two Richmond (06-23) **predate the cutover** and are pre-existing HWM-starvation misses, not this interaction. Only the six with `last_different` below the seed watermark fit this mechanism.

**Attribution to Lane D is inference, not proof.** Which lane inserted a row is unanswerable from the database — see below.

### The diagnostic gap

`applications` has **no ingest-time column**. `last_scraped` and `last_different` are PlanIt's own fields, identical to the microsecond on every row, because PlanIt stamps both when a scrape finds a difference. Nothing records when *we* first stored a row, or which lane stored it. "Which lane got there first, and when" cannot be answered from Postgres at all.

This has now blocked two consecutive investigations (2026-07-16 and 2026-07-17), and in the first it allowed a wrong conclusion to stand for a day.

### Why this is not urgent

The acute phase is self-resolving. Lane D's cursor is at `start_date` ~2026-06-23 and recedes ~7 `start_date` days per calendar day, against a mask edge advancing 1/day — a net ~6/day. It exits Lane A's 90-day mask band around **2026-07-28**, after which the two lanes never overlap again, because Lane D only ever moves backward. Measured exposure since the cutover is zero: 11 of 11 in-zone paid applications notified.

**But the hole reopens the moment anyone** widens `POLLING_LANE_A_MASK_DAYS`, re-seeds Lane D at the head, adds a fifth lane, or runs a second backfill. It closed by luck of timing, not by design. That is what this ADR fixes.

## Decision

### 1. Lane D's window must never overlap Lane A's mask band

Lane A owns `start_date` `[today - POLLING_LANE_A_MASK_DAYS, today]` and notifies. Lane D owns `start_date` strictly older than that and never notifies. The two must not meet.

Clamp Lane D's initial `window_end` to `today - POLLING_LANE_A_MASK_DAYS` rather than `today`, derived from the *same* config value Lane A masks on, so the two cannot drift apart. Lane D's first window becomes `[today - mask - width, today - mask]`.

This makes non-overlap **structural rather than incidental**, in the same spirit as ADR 0042's no-`WithFanOut` guarantee: the unsafe configuration stops being reachable instead of being avoided by care. It is also a no-op for the current Lane D instance, which has already receded past the band — this is a guardrail against recurrence, not a repair.

Records Lane A misses inside the mask band are not orphaned by this: that band is exactly what Lane C sweeps weekly, and Lane C both hydrates *and* notifies.

### 2. Record ingest provenance on `applications`

Add two columns, written on insert and never updated:

- `first_seen_at timestamptz not null default now()` — our clock, not PlanIt's.
- `first_seen_lane text` — `A` | `B` | `C` | `D`.

This is justified independently of the interaction above: it is the column whose absence blocked two investigations, and it turns "did any lane silently absorb something?" from database archaeology into a query. It also gives Lane C a provenance signal to reason about later, should decision 1 ever prove insufficient.

**This reverses ADR 0041's constraint 4** (*"No schema migration. Leave the `poll_state` columns in place and unused, so rollback is a pure image redeploy"*). That constraint was scoped to the cutover, where a pure-redeploy rollback was the point. The cutover has landed and held for two days; the constraint has served its purpose. An additive, nullable-by-default column with a `default now()` is rollback-safe on its own terms — an older image simply ignores it.

## Consequences

**Easier.** Lane provenance becomes a query rather than an inference, so this class of question is answerable in seconds instead of a session. The Lane A/Lane D boundary becomes a stated invariant that a test can pin, rather than a property that happens to hold because of a cursor's current position. Widening the mask, re-seeding the backfill, or adding a lane stops being quietly dangerous.

**Harder.** A schema migration is back on the table, so rollback is no longer purely an image redeploy for the release that carries it. Lane D covers slightly less ground per unit time in exchange (it can never help inside the mask band), which is the correct trade: that band is Lane A's job, with Lane C behind it.

**Explicitly rejected: let Lane C re-notify records Lane D inserted.** This was the first shape considered — flag `first_seen_lane = 'D'` rows as straggler candidates and fan them out. It collides head-on with ADR 0042's constraint (*"a resident must never get a push or email about something we only found by looking backward"*), and defending it would mean arguing about which backward-discovered records are "recent enough" to notify — precisely the judgement call ADR 0042 removed by making the guarantee structural. Decision 1 achieves the same end by keeping the lanes apart, with no such argument to have.

**Explicitly rejected: do nothing.** Defensible on today's numbers — the overlap ends ~2026-07-28 on its own and measured exposure is zero. Rejected because the safety depends on a cursor position rather than on the design, and the failure it guards against is silent by construction: nothing alerts, and at 1-2 notifications a day nobody would spot it by eye.

**Sequencing.** Decision 1 is only meaningful while Lane C works, and Lane C is currently disabled (GH #971, bead tc-tuge8). Land #971 first. Decision 2 is independent and can land at any time.
