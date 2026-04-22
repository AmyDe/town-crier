# 0019. Extract polling into Container Apps Job

Date: 2026-04-04

## Status

Accepted. Narrowed by [ADR 0024](0024-service-bus-only-polling.md) — cron is no longer a polling trigger; it only bootstraps the Service Bus chain when the queue is empty.

## Context

The PlanIt polling service ran as a BackgroundService inside the API container, forcing MinReplicas=1 to keep it alive. With near-zero traffic, this meant paying for a container 24/7 that only needed to do useful work for a few seconds every 15 minutes. ADR 0009 contemplated extraction when workload warranted it.

## Decision

Extract polling into a dedicated Container Apps Job (cron-triggered). The API container reverts to MinReplicas=0 (scale to zero). A new `town-crier.worker` console app runs one poll cycle per invocation and exits.

Key simplifications during extraction:
- Polling schedule prioritisation removed — all authorities polled every run (re-add when scale warrants)
- Health tracking removed — Application Insights provides failure visibility via OpenTelemetry metrics
- File-based poll state replaced with Cosmos DB document persistence

## Consequences

- API container scales to zero when idle — near-zero hosting cost
- Polling job runs on a cron schedule with its own container image and lifecycle
- Two container images to build and deploy (API + worker) instead of one
- Poll state survives job restarts (persisted in Cosmos)
- No scheduling prioritisation — all authorities polled every run, which is fine at current scale
- Health alerting requires App Insights queries rather than in-process monitoring
