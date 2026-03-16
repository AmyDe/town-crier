# 0004. Mock PlanWire.io API for Integration Testing

Date: 2026-03-16

## Status

Accepted

## Context

Town Crier integrates with PlanWire.io as its sole external data provider (see ADR 0003). Integration tests must verify the full request/response cycle — webhook ingestion, API calls, HMAC signature verification — against a realistic PlanWire contract. However, tests must not depend on the live PlanWire service because:

- The free tier is rate-limited (100 req/day), making CI runs fragile.
- Tests must be deterministic and offline-capable.
- New contributors should not need PlanWire API keys to run the test suite.

## Decision

We will build and maintain a **mock PlanWire.io service** — a lightweight HTTP stub that faithfully implements the PlanWire API contract, including webhook payload generation and HMAC-SHA256 signature headers.

The mock will be:
- **Contract-first** — maintained alongside the real PlanWire integration code. When the PlanWire contract changes, both the mock and the integration code are updated together.
- **Containerised** — packaged as a Docker image so it can be composed into the test environment (see ADR 0005).
- **Configurable** — supports canned responses and scenario-based behaviour (e.g., returning errors, rate-limit responses) to test edge cases.

Technology choice (WireMock, custom ASP.NET Minimal API, etc.) will be decided during implementation.

## Consequences

- **Realistic integration coverage** without hitting a live service.
- **Mock maintenance cost** — the mock must track PlanWire's real API contract. Drift is the primary risk, mitigated by periodic contract validation against the live API documentation.
- **No account required** — developers and CI can run all integration tests without PlanWire credentials.
