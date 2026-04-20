# Resumable Pagination Cursor for PlanIt Polling

Date: 2026-04-20

See ADR: [`docs/adr/0021-resumable-pagination-cursor-for-planit-polling.md`](../adr/0021-resumable-pagination-cursor-for-planit-polling.md).

## Problem

PlanIt maintenance events can stamp thousands of applications with the same `last_different` date. Because `different_start` in the PlanIt API is date-truncated, the existing page-cap (PR #265) freezes the HWM on the maintenance date and re-fetches the same first N pages forever. Watch-zone authorities experiencing genuine bursts (consultation deadlines, bulk decision days, reindex events) are affected identically. Manual monitoring is not viable — this must self-heal.

## Design

### Schema change — extend `PollStateDocument`

```csharp
internal sealed class PollStateDocument
{
    public required string Id { get; init; }
    public required string LastPollTime { get; init; }
    public required int AuthorityId { get; init; }

    // Cursor fields (all nullable, absent when no active cursor).
    public string? CursorDifferentStart { get; init; }  // yyyy-MM-dd
    public int? CursorNextPage { get; init; }
    public int? CursorKnownTotal { get; init; }
}
```

All three cursor fields move together — write them as a set, clear them as a set. Update the JSON serializer context accordingly.

### Port change — `IPlanItClient`

Replace the streaming method with a page-level method:

```csharp
public interface IPlanItClient
{
    Task<FetchPageResult> FetchApplicationsPageAsync(
        int authorityId,
        DateTimeOffset? differentStart,
        int page,
        CancellationToken ct);

    Task<PlanItSearchResult> SearchApplicationsAsync(
        string searchText,
        int authorityId,
        int page,
        CancellationToken ct);
}

public sealed record FetchPageResult(
    int PageNumber,
    IReadOnlyList<PlanningApplication> Applications,
    int? Total,
    bool HasMorePages);
```

`HasMorePages = Applications.Count >= DefaultPageSize` (same signal the current loop uses internally).

### Port change — `IPollStateStore`

Add cursor read/write/clear semantics. Prefer a single combined update shape so the invariant "cursor fields move as a set" is encoded in the type system:

```csharp
public interface IPollStateStore
{
    Task<PollState?> GetAsync(int authorityId, CancellationToken ct);

    Task SaveAsync(
        int authorityId,
        DateTimeOffset lastPollTime,
        PollCursor? cursor,
        CancellationToken ct);

    Task<IReadOnlyList<int>> GetLeastRecentlyPolledAsync(
        IReadOnlyList<int> candidateAuthorityIds,
        CancellationToken ct);
}

public sealed record PollState(DateTimeOffset LastPollTime, PollCursor? Cursor);

public sealed record PollCursor(DateOnly DifferentStart, int NextPage, int? KnownTotal);
```

The existing two-method shape (`GetLastPollTimeAsync` / `SaveLastPollTimeAsync`) is replaced. Callers outside the handler (if any) are updated.

### Handler flow — `PollPlanItCommandHandler.HandleAsync`

```
for each authority in sortedIds:
    state = pollStateStore.GetAsync(authorityId)
    lastPollTime = state?.LastPollTime ?? now.AddDays(-1)

    // Cursor validity: only resume if the cursor was recorded against
    // the same date we're about to query.
    startPage = 1
    if state?.Cursor is { } cursor && cursor.DifferentStart == DateOnly.FromDateTime(lastPollTime.UtcDateTime):
        // Overlap by one page for page-shift safety
        startPage = Math.Max(1, cursor.NextPage - 1)

    pagesFetched = 0
    highWaterMark = null
    firstPageTotal = null
    reachedNaturalEnd = false
    capHit = false

    for (page = startPage; pagesFetched < options.MaxPagesPerAuthorityPerCycle; page++):
        result = planItClient.FetchApplicationsPageAsync(authorityId, lastPollTime, page, ct)
        if page == startPage: firstPageTotal = result.Total

        foreach app in result.Applications:
            authorityAppCount++
            highWaterMark = max(highWaterMark, app.LastDifferent)
            // existing: reindex-flood skip, upsert, zone fan-out, notify
            …

        pagesFetched++
        if not result.HasMorePages:
            reachedNaturalEnd = true
            break

    // Persist outcome
    if reachedNaturalEnd:
        pollStateStore.SaveAsync(authorityId, highWaterMark ?? now, cursor: null)
    else if capHit (pagesFetched == MaxPagesPerAuthorityPerCycle):
        // Freeze HWM, save cursor
        pollStateStore.SaveAsync(
            authorityId,
            lastPollTime,                         // HWM unchanged
            cursor: new PollCursor(
                DifferentStart: DateOnly.FromDateTime(lastPollTime.UtcDateTime),
                NextPage: page,                   // the next unfetched page
                KnownTotal: firstPageTotal))
    // Rate-limit and error paths: existing logic, but go via same SaveAsync shape.
```

Existing behaviour preserved:

- Partial-progress HWM tracking on exceptions (polling-partial-progress spec) remains; the cursor is an additive concept.
- Reindex-flood short-circuit (`HasSameBusinessFieldsAs`) remains untouched.
- Cycle termination metrics (`towncrier.polling.cycles_completed`) continue to tag `termination`.

### Telemetry additions

- `towncrier.polling.authority_total` — gauge, per-authority, tagged `cycle.type` and `polling.authority_code`. Emitted once per authority when page 1 is fetched (or first fetched page if resuming).
- `towncrier.polling.cursor_advanced` — counter, +1 per authority when the handler saves a non-null cursor. Tagged `cycle.type`.
- `towncrier.polling.cursor_cleared` — counter, +1 per authority when the handler clears a previously-active cursor (natural end). Tagged `cycle.type`.
- Activity span tags on "Poll Authority": `polling.cursor.next_page`, `polling.authority_total`.

### Rate-limit handling

A 429 mid-pagination is treated the same as a cap: save cursor, do not advance HWM. If `authorityAppCount == 0` on the first page of a resume, behaviour matches the existing "skipped" path (no cursor change).

### Fake updates

- `FakePlanItClient`: replace `FetchApplicationsAsync` stub with a `FetchApplicationsPageAsync` stub that returns pre-seeded pages. Existing helpers (`ThrowForAuthority`, `ThrowAfterYielding`) ported to the new shape.
- `FakePollStateStore`: add `GetAsync` / combined `SaveAsync`; keep `GetLeastRecentlyPolledAsync`. Existing tests migrated to the new shape in lockstep.

## Test strategy

New tests (TUnit, handler level):

1. `Should_StartAtPage1_When_NoCursorExists`
2. `Should_ResumeAtCursorPage_When_CursorMatchesDate` — cursor `nextPage=4`, handler fetches pages 3–5 (overlap -1).
3. `Should_IgnoreStaleCursor_When_DifferentStartDateHasAdvanced` — cursor `differentStart=2026-04-18`, HWM `2026-04-19` → start at page 1.
4. `Should_SaveCursor_When_PageCapHits` — 5-page spike, cap=3. Assert cursor `{date, nextPage=4}` saved, HWM unchanged.
5. `Should_ClearCursor_When_NaturalEndReached` — 2 pages returned, `HasMorePages=false`. Assert cursor cleared, HWM advanced.
6. `Should_SaveCursor_When_RateLimitHitsMidPagination` — 429 on page 2. Assert cursor saved for page 2, HWM unchanged.
7. `Should_ResumeSpike_AcrossMultipleCycles` — 7-page spike, cap=3. Cycle A → cursor page 4, cycle B → cursor page 7, cycle C → natural end + HWM advance.
8. `Should_EmitAuthorityTotal_FromPage1Response` — `FetchPageResult.Total=7200` → metric tagged with that value.
9. `Should_EmitCursorAdvancedCounter_When_CursorSaved`.
10. `Should_EmitCursorClearedCounter_When_CursorClearedAfterActive`.

Infrastructure-level tests:

- `CosmosPollStateStoreTests.Should_RoundTripCursor` — write state with cursor, read back, assert equality.
- `CosmosPollStateStoreTests.Should_RoundTripState_WithoutCursor` — backward compat with documents missing the cursor fields.
- `PlanItClientTests.Should_ReturnHasMorePagesTrue_When_FullPageReturned` and `…False_When_PartialPage`.

## Rollout

1. Implement schema extension + serializer context update.
2. Introduce new port shapes (`PollState`, `PollCursor`, `FetchPageResult`) and port methods. Keep this PR compile-clean by updating all callers in one go.
3. Reimplement `PlanItClient.FetchApplicationsPageAsync`.
4. Reimplement handler with cursor branch + new metrics.
5. Update fakes and tests.
6. No feature flag — cursor fields are nullable and backward-compatible; existing documents continue to work unchanged. Old-image replicas read the new schema harmlessly (cursor fields ignored).
7. Post-merge: watch `towncrier.polling.cursor_advanced` vs `cursor_cleared` in App Insights. `advanced > cleared` for an extended period on a single authority = a persistent spike (expected behaviour). `advanced` without eventual `cleared` for >48h on one authority = investigate (cursor leak).

## Out of scope

- Locking to prevent concurrent writers — single-job-replica deployment makes this unnecessary.
- Separate backfill worker / queue.
- Alerting on maintenance-spike detection. `authority_total` telemetry provides the raw signal; alerts can be layered on later.
- Changing the reindex-flood skip logic (`HasSameBusinessFieldsAs`) — unchanged.
