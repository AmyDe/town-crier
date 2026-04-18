# Seed Polling — Alternating Watched / Full-Set Sync

Date: 2026-04-18

## Problem

The poll worker currently syncs only authorities that contain at least one active watch zone (`WatchZoneActiveAuthorityProvider`). At today's scale that is ~8 authorities, all of which poll every 15 minutes. The remaining ~409 UK local planning authorities are never synced.

When a new user signs up and creates their first watch zone in a never-synced authority, the map and applications views are empty until the next poll cycle discovers data. We want the views to be populated with recent activity (last few days) before the user's first interaction.

## Goals

- On signup, the user's map/applications view shows recent applications for their authority without waiting for a reactive backfill.
- Freshness target for unwatched authorities: **at most a few days stale**.
- Do not regress notification behaviour for watched authorities beyond 30-minute freshness.
- Do not pressure PlanIt (a free, hobby-run API) beyond what the current rate-limit tolerance allows.
- Keep the handler simple — no intra-cycle priority queues or budget arithmetic.

## Non-Goals

- Reactive / lazy backfill on watch zone creation.
- Retroactive notification matching for applications ingested before a zone existed (the existing `zone.CreatedAt > application.LastDifferent` filter is preserved).
- Configurable or per-tier freshness SLAs.

## Observed Baseline (2026-04-17 → 2026-04-18)

Measured from App Insights over the last 24h:

| Metric | Value |
|--------|-------|
| Authorities polled per cycle | ~8 (760 / 96 cycles) |
| Applications ingested per day | 40,052 |
| Rate-limit (429) hits | 0 |
| Avg cycle duration | ~45s |

All 8 watched authorities are polled on every cycle with comfortable headroom against PlanIt's rate-limit ceiling.

## Design

### Alternating cycles

The cron schedule (`*/15 * * * *`) and the `PollPlanItCommandHandler` are unchanged. On each invocation the worker selects which set of authorities to feed into the handler based on the current UTC minute:

```
(utcNow.Minute % 30) < 15   →   watched cycle   (ids from WatchZoneActiveAuthorityProvider)
otherwise                    →   seed cycle      (all 417 authority ids)
```

The modulo tolerates up to ~15 minutes of Container Apps Job startup drift — both minute 0 and minute 14 resolve to "watched", and both 15 and 29 resolve to "seed". We do not require the job to fire at exact 00/15/30/45 boundaries.

Effective cadence:

- Watched authorities: polled every 30 minutes (down from 15).
- Seed authorities: ~8 polls per seed cycle × 48 seed cycles/day = ~384 seed polls/day → full 417-authority rotation in **~26 hours**.

### Natural LRU separation

Seed cycles reuse the existing `GetLeastRecentlyPolledAsync` sort. Watched authorities polled 15 minutes earlier will have the freshest `lastPollTime` in the PollState container, so they sink to the bottom of the seed queue. Unwatched authorities (either never polled or polled days ago) rise to the top and get the seed cycle's request budget. No explicit "exclude watched" filter is required.

### Components

1. **`IWatchZoneActiveAuthorityProvider`** and **`IAllAuthorityIdProvider`** — two new marker interfaces in `town-crier.application.Polling`. Each extends `IActiveAuthorityProvider` (same contract, different identity) so the alternating provider can depend on each distinctly via DI without ambiguous resolution.

2. **`AllAuthorityIdProvider`** — new class in `town-crier.infrastructure.Authorities` implementing `IAllAuthorityIdProvider`. Adapts the existing `StaticAuthorityProvider` by projecting its list down to `IReadOnlyCollection<int>` of authority IDs.

3. **`WatchZoneActiveAuthorityProvider`** — existing class, updated to additionally implement `IWatchZoneActiveAuthorityProvider`. No behavioural change.

4. **`CycleAlternatingAuthorityProvider`** — new class in `town-crier.application.Polling`. Implements `IActiveAuthorityProvider`. Takes three dependencies:
   - `IWatchZoneActiveAuthorityProvider`
   - `IAllAuthorityIdProvider`
   - `TimeProvider`

   On `GetActiveAuthorityIdsAsync`, computes the cycle type from `TimeProvider.GetUtcNow().Minute` and delegates to the appropriate inner provider.

5. **DI wiring change** — `IActiveAuthorityProvider` now resolves to `CycleAlternatingAuthorityProvider`. `IWatchZoneActiveAuthorityProvider` resolves to `WatchZoneActiveAuthorityProvider`, `IAllAuthorityIdProvider` resolves to `AllAuthorityIdProvider`.

The handler, its tests, and `FakeActiveAuthorityProvider` are unchanged.

### First-sync history window

When a never-polled authority is polled for the first time (via a seed cycle), the handler falls back to `lastPollTime ??= now.AddDays(-1)`. That 24-hour window is retained — no change. After the first full rotation (~26h), every authority has ~24h of data. After day 2, each has ~48h, and so on. The catalogue deepens naturally over time, which is acceptable given we expect paying users no earlier than ~1 month from now.

### Telemetry

Add a `cycle.type` tag (`"watched"` | `"seed"`) to:

- The poll cycle trace span in `Program.cs`.
- The `authorities_polled`, `applications_ingested`, `authorities_skipped`, and `rate_limited` counters.

This allows dashboard and alerting queries to separate seed-cycle health from watched-cycle health without guessing from cycle wall-clock time.

## Failure Modes

- **A watched-cycle run fails or rate-limits** — watched authorities wait 60 minutes until the next watched slot. Acceptable at current scale; no SLA regression because none is promised.
- **A seed-cycle run fails** — seed progress is delayed by 30 minutes. Catalogue growth is inherently best-effort; no user-facing impact.
- **PlanIt rate-limit behaviour changes** — existing `HttpRequestException { StatusCode: TooManyRequests }` handling in the handler continues to break the loop cleanly and records the `rate_limited` counter.

## Testing

### New tests

- `CycleAlternatingAuthorityProviderTests` — verifies cycle-type routing at minute boundaries 0, 14, 15, 29, 30, 44, 45, 59, using a `FakeTimeProvider`. Asserts the correct inner provider is invoked and its result returned unchanged.
- `AllAuthorityIdProviderTests` — verifies the full 417-authority ID set is returned and matches the count in the embedded JSON.

### Unchanged tests

- `PollPlanItCommandHandlerTests`, `PollPlanItCommandHandlerMetricsTests`, `PollPlanItCommandHandlerTracingTests` — continue using `FakeActiveAuthorityProvider`. No assertions about cycle type at the handler level.
- `WatchZoneActiveAuthorityProviderTests` — unchanged.

### Integration

No new integration tests. The behaviour is visible end-to-end via App Insights once deployed (look for alternating cycle types and seed-cycle coverage growing over a 24-hour window).

## Consequences

- **Simpler:** the handler is unchanged; the cycle-type decision is a single modulo in one new class.
- **Simpler:** natural LRU self-organisation avoids an explicit "watched vs unwatched" filter for seed cycles.
- **Slower notifications for watched authorities** — 30-min freshness instead of 15-min. Acceptable at current scale (no SLA) but a point of vigilance if a notification-latency commitment is later introduced.
- **Storage grows for authorities nobody watches.** At ~40K apps/day currently restricted to 8 authorities, expanding to 417 will raise the daily ingestion rate. Cosmos serverless cost remains in single-digit dollars/month at this volume (memo 0001) and well within the 1TB container cap.
- **More visible PlanIt load.** Seed cycles add ~384 polls/day (previously zero) — still within the observed rate-limit headroom, but worth monitoring for the first week post-deployment.
