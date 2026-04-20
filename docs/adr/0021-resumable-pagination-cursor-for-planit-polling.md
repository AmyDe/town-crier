# 0021. Resumable Pagination Cursor for PlanIt Polling

Date: 2026-04-20

## Status

Accepted

## Context

Seed-poll cycles alternate with watched-poll cycles every 15 minutes (ADR 0019, seed-polling spec). The handler advances a per-authority high-water mark (HWM) on `last_different` so the next cycle queries `different_start=HWM.Date` and picks up from where it left off (polling-partial-progress spec).

PR #265 (2026-04-20) added a voluntary page cap (`Polling:MaxPagesPerAuthorityPerCycle`, default 3) so a backlogged authority cannot monopolise the rate budget mid-pagination. Telemetry confirms the cap works as intended — per-authority processing dropped from 280s → 65s, seed success rate went 0% → 96%.

**However, the page cap exposes a correctness gap when combined with PlanIt maintenance events.** PlanIt occasionally performs bulk updates that stamp thousands of applications with the same `last_different` date (observed yesterday: one authority returned 7,200+ apps with a single maintenance date). Because PlanIt's `different_start` parameter is date-truncated:

1. Cycle N caps at page 3 of the spike (~300 apps), advances HWM to `X` (still the spike's date).
2. Cycle N+1 queries `different_start=X.Date`, receives the same pages 1–3, caps again, HWM is still `X`.
3. Progress is zero. The HWM cannot escape the maintenance date.

This is not a theoretical concern. The same failure mode will occur on any **watched-zone authority** that experiences a genuine burst (planning consultation deadlines, bulk decision days, reindex events), and users' notifications would be silently delayed until the spike clears — which may be never. Manual intervention is the only current remedy, which is unworkable for a solo developer.

## Decision

Introduce a **page-level cursor** to the existing per-authority poll state so successive cycles resume at the next unprocessed page within a date, rather than restarting from page 1. Cursor clears naturally when the authority reaches end-of-data.

**State model:** extend `PollStateDocument` (already per-authority in the `PollState` Cosmos container) with three nullable fields:

- `CursorDifferentStart` — the date we were paginating against (ISO `yyyy-MM-dd`).
- `CursorNextPage` — the next page number to fetch.
- `CursorKnownTotal` — PlanIt's reported `total` from the page-1 response (captured for logging and telemetry; not required for correctness).

**Client contract:** replace the streaming `FetchApplicationsAsync` with a page-level method `FetchApplicationsPageAsync(authorityId, differentStart, page, ct)` that returns `{ Applications, Total, HasMorePages }`. The handler drives the page loop, tracks state, and writes the cursor when it stops short.

**Handler logic:**

- **Start:** if a cursor exists and `CursorDifferentStart == lastPollTime.Date`, resume from `CursorNextPage`. Otherwise start at page 1.
- **Natural termination** (`HasMorePages == false`): clear the cursor, advance HWM to the highest `LastDifferent` streamed.
- **Cap or rate-limit termination** with more pages remaining: save cursor `{ differentStart, nextPage, knownTotal }`, **do not advance HWM** for this authority. The next cycle resumes exactly where this one left off.
- **Overlap on resume:** refetch `nextPage - 1` as a one-page safety margin against PlanIt page-boundary shifts during ongoing mutations. Applications are already idempotent upserts (`PlanningApplicationRepository.UpsertAsync`), so duplicate reads are harmless.

**Telemetry additions:**

- `towncrier.polling.authority_total` (gauge, tagged `cycle.type`) — `PlanItResponse.Total` captured from page 1. Makes spike authorities observable.
- `towncrier.polling.cursor_advanced` (counter, tagged `cycle.type`) — emitted on cursor save, so spike-recovery progress is visible.
- Per-authority activity span tag `polling.cursor.next_page` added when a cursor is active.

## Consequences

### Easier

- Spike authorities automatically drain over successive cycles (a 7,200-app spike takes ~24 seed cycles ≈ 12 hours — slow but unattended).
- Both seed and watched cycles benefit uniformly; the same cursor applies to either path.
- No new Cosmos container; the cursor lives on the existing per-authority poll state document.
- Page-level client API is easier to test than a streaming enumerable (handler can use a fake that returns canned `FetchPageResult`s).

### Harder

- `IPlanItClient.FetchApplicationsAsync` is replaced. One caller (`PollPlanItCommandHandler`) needs updating plus the existing streaming tests and fakes.
- Cursor lifecycle is a new invariant — mis-handling (e.g. forgetting to clear on natural end) would cause the handler to loop on a stale cursor. Mitigation: invariant tests that pair each cursor write path with a clear path.
- Handler gains a second Cosmos write per cycle for authorities with active cursors (one cursor upsert plus the existing HWM upsert). Cost is negligible but notable.

### Not addressed

- Concurrent-authority locking (seed and watched cycles cannot currently run simultaneously in the same job replica, so this is not a real concern today).
- A separate backfill queue for historic seed catchup — rejected as over-engineering; the cursor approach handles it uniformly.
- Detection-and-alerting on maintenance spikes themselves — a `CursorKnownTotal` threshold could feed a future alert, but is out of scope here.
