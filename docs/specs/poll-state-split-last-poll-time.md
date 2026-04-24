# Split `LastPollTime` and `HighWaterMark` in `PollState`

## Problem

Authorities with no recent applications — "quiet" LPAs like authority 156 (6d 14h behind as of 2026-04-24 17:22Z) — get selected as the stalest authority on every cycle, even immediately after being polled. The scheduler re-polls them first each cycle, finds nothing new, and the age metric grows linearly with wall-clock time. Other authorities that also need polling are starved.

## Root cause

`PollState.LastPollTime` is doing double duty:

1. **PlanIt cursor** — used as `different_start` when fetching pages (`PollPlanItCommandHandler.cs:138`).
2. **Freshness sort key** — used by `GetLeastRecentlyPolledAsync` to order authorities for the next cycle (`CosmosPollStateStore.cs:86`).

On a successful poll, the handler writes `highWaterMark ?? now` to `LastPollTime`, where `highWaterMark = max(application.LastDifferent)` seen in the response (`PollPlanItCommandHandler.cs:164, 290`). For a quiet authority whose newest application is 6.6 days old, `highWaterMark` is 6.6 days old, so `LastPollTime` also lands 6.6 days old — then the next cycle sorts the same authority back to the top. The poll is a no-op with respect to its own scheduling.

The `highWaterMark ?? now` fallback only fires when an authority has **zero** applications at all (never happens in practice for a real LPA). Any authority with at least one historical app — regardless of age — is stuck.

## Proposed design

Split the two concepts into separate fields on `PollState`:

```csharp
public sealed record PollState(
    DateTimeOffset LastPollTime,   // when we last ran a poll for this authority (scheduling)
    DateTimeOffset HighWaterMark,  // max LastDifferent ingested (PlanIt cursor)
    PollCursor? Cursor);
```

- `LastPollTime` is always set to `now` on any non-skip branch (successful poll, cap-hit save, rate-limited mid-cycle save).
- `HighWaterMark` replaces the current `LastPollTime` as the PlanIt `different_start` cursor and as the anchor for `PollCursor.DifferentStart`.
- `GetLeastRecentlyPolledAsync` sorts by `LastPollTime` ascending — quiet authorities immediately drop to the back of the queue after being polled.
- The existing `towncrier.polling.oldest_hwm_age_seconds` metric continues to report `now - LastPollTime` for the stalest authority. Semantically the metric becomes "longest time since last poll" — a true backlog/scheduling signal. This is what the name already implies; today it's actively misleading.

## Schema migration

`PollStateDocument` in `CosmosPollStateStore.cs` gains a `HighWaterMark` field. Existing documents:
- On read, if `HighWaterMark` is missing, fall back to `LastPollTime` (the old conflated value) — correct for cursor behaviour.
- On the next write, both fields are populated; no batch migration needed.

## Telemetry / verify-polling impact

- `verify-polling` invariant 3e (oldest HWM age) becomes a purer signal: any age exceeding `authorities_polled_per_cycle × cycle_interval` genuinely means the cycle isn't keeping up. Today the metric is polluted by quiet LPAs and can't be used as a backlog alarm.
- No metric rename needed (the name was always about scheduling lag, not data age).

## Test plan

1. Unit test (TUnit) on `PollPlanItCommandHandler`:
   - Authority returns 5 apps, newest `LastDifferent = T-7days`. Assert `SaveAsync` called with `LastPollTime = now`, `HighWaterMark = T-7days`.
   - Authority returns 0 apps. Assert `LastPollTime = now`, `HighWaterMark` unchanged (or `DateTimeOffset.MinValue` on first poll).
   - Cap-hit mid-pagination. Assert `LastPollTime = now`, `HighWaterMark` frozen at last-observed value.
   - Rate-limited mid-pagination with partial progress. Same as cap-hit.
2. Unit test on `CosmosPollStateStore.GetLeastRecentlyPolledAsync`:
   - Two authorities: A polled 1 min ago with HWM from 7 days ago; B polled 1 hour ago with HWM from 1 hour ago. Assert B sorts before A.
3. Backward-compat test:
   - Seed a `PollStateDocument` with only `LastPollTime` (no `HighWaterMark`). `GetAsync` returns a `PollState` whose `HighWaterMark` equals the old `LastPollTime`.
4. Integration smoke: run the cycle twice against a stub PlanIt that returns a stale app, assert the second cycle does NOT select the same authority first.

## Non-goals

- Not changing the PlanIt cursor semantics (`different_start` still uses `HighWaterMark`).
- Not introducing per-authority cadence tuning — all authorities still eligible every cycle, just sorted correctly.
- Not touching bootstrap or lease CAS logic.
