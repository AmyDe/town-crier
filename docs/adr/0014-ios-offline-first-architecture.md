# 0014. iOS Offline-First Architecture

Date: 2026-03-27

## Status

Accepted

## Context

Town Crier's iOS app must remain useful when network connectivity is degraded or unavailable. Users may open the app on public transport, in areas with poor signal, or when their device is in airplane mode. Planning application data viewed recently should still be accessible, and the app should degrade gracefully rather than showing error screens.

Additionally, the app needs crash reporting for production diagnostics and network state awareness to make intelligent data-fetching decisions. These are capability-level architectural choices that affect how the app behaves under adverse conditions.

## Decision

### Offline Caching via Decorator Pattern

An `OfflineAwareRepository` decorator wraps any `PlanningApplicationRepository` implementation. The decorator manages a cache layer transparently:

1. **Online + cache fresh**: return cached data without network call
2. **Online + cache stale/empty**: fetch from remote, cache result, return fresh data
3. **Offline + cache available**: return stale cached data with `DataFreshness.stale` signal
4. **Offline + no cache**: throw `DomainError.networkUnavailable`

Cache entries use a `CacheEntry<T>` value type with a configurable TTL (default 900 seconds). The `DataFreshness` enum (`.fresh` / `.stale`) is propagated to the UI layer, enabling stale-data banners without blocking the user.

### In-Memory Cache with Actor Isolation

The cache store (`InMemoryApplicationCacheStore`) is implemented as a Swift **Actor**, providing thread-safe access without manual locks or dispatch queues. Cache entries are keyed by authority code. No on-device persistence (SwiftData, Core Data, SQLite) is used — the cache is populated on each app session from the API and retained in memory.

This is a deliberate simplicity choice: planning data changes frequently, sessions are typically short, and the API is the source of truth. Persistent offline storage may be added later if usage patterns demand it.

### Connectivity Monitoring

`NWPathConnectivityMonitor` wraps Apple's **Network.framework** `NWPathMonitor` to provide real-time connectivity state via the `ConnectivityMonitor` protocol. The offline-aware repository checks connectivity before deciding whether to attempt network requests, avoiding unnecessary timeouts.

### Crash Reporting via MetricKit

Apple's **MetricKit** framework (`MXMetricManagerSubscriber`) is used for crash diagnostics rather than a third-party service (Sentry, Firebase Crashlytics, Bugsnag). `MetricKitCrashReporter` implements the `CrashReporter` protocol and logs crash reports (signal, termination reason, VM region info) to the OS unified logger.

This eliminates a third-party dependency and associated privacy implications. The trade-off is delayed delivery (crash reports arrive 0–24 hours post-crash) and no aggregation dashboard. Acceptable at the current scale; a third-party service can be swapped in via the `CrashReporter` protocol if needed.

## Consequences

- **Better UX under poor connectivity** — users see cached data with a staleness indicator instead of error screens or loading spinners.
- **Simpler data layer** — in-memory cache avoids the complexity of on-device database schemas, migrations, and sync conflicts. The decorator pattern keeps the caching concern separate from repository logic.
- **No persistent offline data** — closing and reopening the app with no connectivity shows empty state until the network returns. This is acceptable for the current use case but limits true offline-first capability.
- **Actor-based thread safety** — eliminates a class of concurrency bugs in cache access, leveraging Swift 6.0 strict concurrency checking.
- **MetricKit limitations** — no real-time crash alerting or dashboards. Crash data is delayed and requires manual log inspection. The `CrashReporter` protocol provides a swap path to a third-party service when scale demands it.
