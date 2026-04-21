# 0012. Dynamic Polling Prioritisation by Watch Zone Density

Date: 2026-03-17

## Status

Superseded by [0019](0019-extract-polling-to-container-apps-job.md)

## Context

ADR 0006 established a 15-minute polling cycle against the PlanIt API to ingest new planning applications. With 417 local planning authorities (LPAs) in England, polling every authority on every cycle is wasteful — most authorities have few or no active watch zones, meaning we spend API calls and compute time fetching data that no user will see.

We needed a strategy to allocate polling bandwidth proportionally to user demand while keeping the system simple and avoiding external scheduling infrastructure.

## Decision

We introduce a density-based polling schedule that tiers authorities by the number of active watch zones they contain:

| Priority | Criteria | Polling Cadence |
|----------|----------|-----------------|
| **High** | Zone count >= `HighThreshold` | Every cycle |
| **Normal** | Zone count >= `LowThreshold` | Every 2nd cycle |
| **Low** | Zone count < `LowThreshold` | Every 4th cycle |

The schedule is encapsulated in a `PollingSchedule` domain value object that is recalculated at the start of each polling cycle from current watch zone counts. Thresholds are configured via `PollingScheduleConfig` (injected at startup). The `ShouldPollInCycle(authorityId, cycleNumber)` method determines whether a given authority should be polled in the current cycle.

Efficiency metrics (authorities polled, skipped, total) are logged each cycle for observability.

## Consequences

- **Simpler:** No external job scheduler or per-authority cron configuration — the priority logic is a pure domain calculation with no infrastructure dependencies.
- **Simpler:** Low-activity authorities are still polled (every 4th cycle = ~60 minutes), so new applications are never missed entirely — just slightly delayed.
- **Harder:** Data freshness is no longer uniform. Users watching a low-priority authority may see applications up to 60 minutes after ingestion vs. 15 minutes for high-priority ones. This is acceptable at current scale but may need revisiting if we introduce SLA guarantees per subscription tier.
- **Harder:** Thresholds need tuning as the user base grows. The current thresholds are suitable for early-stage density distributions but may need adjustment or replacement with a percentile-based scheme.

## Amendments

### 2026-04-21
- Status changed to **Superseded by [ADR 0019](0019-extract-polling-to-container-apps-job.md)**. When polling was extracted from the API into a dedicated Container Apps Job (2026-04-04), the `PollingSchedule` value object and `ShouldPollInCycle()` logic were removed. The current worker polls all active authorities every run. Authority selection is instead handled by `ICycleSelector` / `IActiveAuthorityProvider` / `IWatchZoneActiveAuthorityProvider`, which alternate between seed and watched-zone cycles (see the seed-polling and resumable-cursor specs). Density-based prioritisation may return if scale warrants, but the original decision no longer reflects the codebase.
