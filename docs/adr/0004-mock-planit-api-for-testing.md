# 0004. Mock PlanIt API for Integration Testing

Date: 2026-03-16

## Status

Superseded by [0016](0016-test-infrastructure-strategy.md)

## Context

Town Crier integrates with PlanIt (planit.org.uk) as its primary data provider via a polling service (see ADR 0006). Integration tests must verify the full request/response cycle — polling, change detection, and application ingestion — against a realistic PlanIt API contract. However, tests must not depend on the live PlanIt service because:

- PlanIt's rate limits are unpublished and enforced by IP, making CI runs fragile.
- Tests must be deterministic — live data changes constantly, so assertions against it are inherently flaky.
- PlanIt is run by a single maintainer and should not be loaded by CI pipelines.

## Decision

We will build and maintain a **mock PlanIt service** — a lightweight HTTP stub that faithfully implements the PlanIt API contract, seeded with a small, fixed dataset captured from the real PlanIt API.

### Test Data

Fixture data is captured once from the real PlanIt API (a handful of authorities, a few dozen applications across various statuses) and stored as **raw C# string literals** directly in the mock service code. No separate fixture files, no seed scripts — the data lives in code alongside the mock endpoints that serve it.

When PlanIt's schema drifts, we update the string literals and the integration code together in a single code change.

This approach gives us:

- **Realistic data shapes** — real field names, value formats, and edge cases that hand-crafted fixtures might miss.
- **Deterministic test runs** — tests always run against the same hardcoded data, so assertions are stable.
- **No live dependency at test time** — CI and local test runs never contact PlanIt, avoiding rate limiting and network flakiness.
- **Simple maintenance** — no fixture files to manage or seed scripts to run. Schema drift is fixed with a code change.

### Mock Service

The mock will be:

- **Contract-first** — maintained alongside the real PlanIt integration code. When the PlanIt contract changes, both the mock and the string literals are updated together.
- **Containerised** — packaged as a Docker image so it can be composed into the test environment (see ADR 0005).
- **Configurable** — supports scenario-based behaviour (e.g., returning 429 rate-limit responses, empty result sets, large page sizes) to test edge cases beyond the canned data.

Technology choice (WireMock, custom ASP.NET Minimal API, etc.) will be decided during implementation.

## Consequences

- **Realistic integration coverage** without hitting a live service — test data comes from real PlanIt responses, not hand-crafted guesses.
- **Mock maintenance cost** — when PlanIt's API contract changes, the string literals and integration code must be updated together. This is a straightforward code change, not a separate process.
- **No external dependency at test time** — developers and CI can run all integration tests fully offline.
