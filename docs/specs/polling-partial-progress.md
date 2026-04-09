# Polling Partial Progress and Metrics Accuracy

Date: 2026-04-09

## Problem

The poll handler treats per-authority pagination as atomic: either all pages succeed (save poll state + emit metrics) or nothing is recorded. But `FetchApplicationsAsync` is a streaming `IAsyncEnumerable` where a 429 exception propagates out of the `await foreach` on page N, after pages 1 through N-1 have already been upserted to Cosmos.

### Observed impact (production, 2026-04-09)

- Authority 52 is polled every 15 minutes, fetching ~800-1200 applications over ~48 seconds across 8-12 pages before hitting a 429
- Applications ARE written to Cosmos (~18,800 RU per cycle) but `authorities_polled` and `applications_ingested` metrics never emit
- `SaveLastPollTimeAsync` never fires, so `GetLeastRecentlyPolledAsync` returns authority 52 every cycle (groundhog day)
- Dashboard correctly shows zero sync successes / zero applications ingested — the metrics are accurate reflections of a code bug, not a metrics pipeline problem

### Root cause

```
try {
    await foreach (app in FetchApplicationsAsync...)    // 429 thrown mid-iteration
        upsert(app)                                      // ~800 apps already written

    SavePollState(authorityId, now)                      // never reached
    AuthoritiesPolled.Add(1)                             // never reached
    ApplicationsIngested.Add(authorityAppCount)          // never reached
}
catch (429) {
    AuthoritiesSkipped.Add(1)
    break
}
```

### Three compounding issues

1. **Groundhog day** — poll state never advances, so the same authority is retried with the same date range every cycle
2. **Invisible work** — ~800 apps upserted per cycle with zero metric emission
3. **No test coverage** — `FakePlanItClient.ThrowForAuthority` throws before yielding anything; mid-pagination 429s are untested

## Design

### Change 1: Switch polling to ascending sort order

Currently: `sort=-last_different` (descending — newest first). With a mid-pagination 429 on page 8, we've processed pages 1-7 (the newest 700 apps) and missed the oldest tail. There's no good resume point.

Change to: `sort=last_different` (ascending — oldest first). Now pages 1-7 contain the oldest 700 apps. The high-water mark (latest `LastDifferent` from what we processed) is a precise resume point — next cycle picks up exactly where we left off.

**Verified**: PlanIt supports ascending sort. Tested with `sort=last_different` on authority 52 — returns 6,722 results in chronological order with correct pagination.

The search endpoint stays `sort=-last_different` (descending) — only the polling URL changes.

### Change 2: Track high-water mark and save partial progress

Track the latest `LastDifferent` from ingested applications as they stream through. After the loop — whether it completes normally, 429s, or errors — if any applications were ingested, save the high-water mark as poll state.

With ascending sort, the high-water mark precisely represents "where we got to" in chronological order.

### Change 3: Restructure handler — metrics fire regardless of exit path

Move metrics and poll-state logic out of the success-only path:

```
for each authority:
    authorityAppCount = 0
    highWaterMark = null

    try:
        foreach app in FetchApplicationsAsync(...):
            upsert(app)
            notify(app)
            authorityAppCount++
            highWaterMark = max(highWaterMark, app.LastDifferent)
    catch 429:
        RateLimited.Add(1)
        rateLimited = true
    catch other:
        log error

    // Always record duration
    AuthorityProcessingDuration.Record(elapsed)

    // Record actual work done (partial or full)
    if authorityAppCount > 0:
        AuthoritiesPolled.Add(1)
        ApplicationsIngested.Add(authorityAppCount)
        SavePollState(authorityId, highWaterMark)
    else:
        AuthoritiesSkipped.Add(1)

    if rateLimited: break
```

### Change 4: Extend FakePlanItClient for mid-pagination failures

Add `ThrowAfterYielding(authorityId, count, exception)` — yields `count` applications then throws. New test cases:

- `Should_SavePollState_When_429HitAfterSomeApplicationsIngested`
- `Should_EmitApplicationsIngested_When_429HitMidPagination`
- `Should_ResumeFromHighWaterMark_When_PreviousCycleWasPartial`
- `Should_AdvanceToNextAuthority_When_PreviousAuthorityPartiallyCompleted`

## Implementation order

1. Add `ThrowAfterYielding` to `FakePlanItClient`
2. Write failing tests for partial-progress scenarios (red)
3. Restructure handler with high-water mark tracking (green)
4. Switch `sort=-last_different` to `sort=last_different` in polling URL only
5. Verify all existing tests still pass

## Future follow-up

**Continue to next authority on 429** — currently any 429 breaks the entire authority loop. We could save partial progress and attempt the next authority, breaking only after N consecutive 429s (heuristic for global rate limit). Separate bead.

## Expected impact

| Scenario | Before | After |
|---|---|---|
| Mid-pagination 429 (current prod) | Re-fetches same 800 apps forever, 0 metrics | Advances past them, accurate counts |
| First-request 429 (no apps fetched) | Skipped, no poll state change | Same — skip, no change |
| Full success | Save `now`, emit metrics | Save high-water mark, emit metrics |
| Catchup from 30-day lookback | Processes newest first, gets stuck on 429 | Processes oldest first, resumes on 429 |
