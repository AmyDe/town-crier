# 0024. Service Bus-only polling

Date: 2026-04-22

## Status

Accepted

Narrows [ADR 0019](0019-extract-polling-to-container-apps-job.md) — 0019 introduced the cron-triggered Container Apps Job model as the sole polling mechanism; this ADR keeps 0019's job model but removes the cron path as a polling trigger, leaving cron only to bootstrap the Service Bus chain when it goes silent.

## Context

PRs #275, #276, #278, #280, #283 and #285 built an adaptive Service Bus-coordinated polling chain, now operational in prod. One event-triggered `poll` Container Apps Job consumes each trigger message, runs `PollPlanItCommandHandler`, publishes the next trigger with a scheduled enqueue time, then acks. Publish-before-ack ordering plus a Cosmos lease guard keep the chain self-healing under individual replica crashes.

Alongside it, a cron-triggered `poll-safety-net` job ran `*/30 * * * *` doing two things:

1. A full `PollPlanItCommandHandler` cycle (the legacy timer-based poll).
2. `PollTriggerBootstrapper.TryBootstrapAsync` — probes the trigger queue and publishes a seed message if empty.

Now that the SB chain is trusted, the full-poll half of the safety-net is redundant: it duplicates the work the chain already does, competes for the same Cosmos lease (and usually loses), costs a 600-second replica timeout every 30 minutes, and muddies telemetry by having two paths call the same handler. The bootstrap half remains valuable as a recovery mechanism for the rare case where the chain dies entirely (e.g. max-delivery-count exhaustion moves the only live message to DLQ, Service Bus maintenance, or a manual queue purge).

Dev has never been on the SB chain — `ServiceBusPollingInfra` is provisioned only when `env == "prod"`. Its cron `poll` job ran every 3 hours purely to keep Cosmos data fresh for manual testing. The intention going forward is to stop dev polling entirely and, as a separate future feature, back-fill dev Cosmos from prod applications once per day.

The polling-handler-soft-budget spec (2026-04-22) flagged this direction: *"Timer mode will be removed in a follow-up bead once the SB-coordinated loop is stable; the safety-net job will then become a bootstrap-only job (probe queue → publish seed if empty), not a full poll."* This ADR records that decision.

## Decision

1. **Prod polling runs only through the Service Bus chain.** The event-triggered `poll` job (`WORKER_MODE=poll-sb`) remains the single path that calls `PollPlanItCommandHandler`. No other entry point invokes the handler.
2. **Cron's role narrows to bootstrap.** The former `poll-safety-net` job becomes `poll-bootstrap` (`WORKER_MODE=poll-bootstrap`) on the same `*/30 * * * *` schedule with a reduced 120-second replica timeout. It runs only `PollTriggerBootstrapper.TryBootstrapAsync` — probe the queue, publish a jittered seed if empty, otherwise abandon and exit. A failed probe or publish is not a worker failure.
3. **Dev loses its poll job.** The dev-only cron `poll` job is removed. Dev Cosmos retains existing data but ingests no new applications until the future prod→dev backfill feature lands.
4. **`WORKER_MODE` fails fast when unset.** The `town-crier.worker` entry point drops its `?? "poll"` default; deployments are required to set `WORKER_MODE` explicitly. Unknown or unset values log `UnknownWorkerMode` and exit 1.

The `poll-sb` orchestrator, `PollPlanItCommandHandler`, `PollNextRunScheduler`, `PollTriggerBootstrapper`, and the rest of the polling infrastructure are unchanged. This ADR is purely about which entry points invoke which paths. [ADR 0012](0012-dynamic-polling-prioritisation.md) and [ADR 0021](0021-resumable-pagination-cursor-for-planit-polling.md) remain in force — both describe the polling algorithm, which is untouched.

## Consequences

### Easier

- One canonical polling path in prod. Dashboards and traces only need to reason about `poll-sb`; the two-paths-calling-one-handler ambiguity is gone.
- Lower steady-state cost. The safety-net's 600-second replica slot every 30 minutes is replaced by a ~1-second probe with a 120-second timeout.
- Bootstrap is cheap enough to keep running on a tight schedule, so the chain's worst-case recovery window is bounded by `(max deliveries × lock duration) + (time-to-next-cron-tick) ≈ 50 + 30 = 80 minutes`.
- Deployments can't silently fall through to a deleted mode — an unset `WORKER_MODE` is now a fail-fast, not a landmine.

### Harder

- If both the SB chain and Service Bus itself are unavailable at the same time, polling pauses until Service Bus recovers. The failure modes for the bootstrap probe (SB outage, identity/RBAC misconfiguration) are also what would break the `poll-sb` chain, so a simultaneous failure is visible rather than silent, but recovery does hinge on Service Bus being healthy.
- Dev Cosmos goes stale until the prod→dev backfill feature lands. Manual testers see whatever data was ingested before this change.
- The safety-net job's dual role (poll + bootstrap) is gone; anything that relied on the cron side as a polling fallback now has to fail over to the SB chain's own self-healing.

### Not addressed

- **Prod→dev Cosmos backfill.** Out of scope; will be a separate bead when dev staleness becomes painful.
- **Service Bus polling infra in dev.** Rejected — would provision SB infrastructure that services essentially no traffic. The backfill feature is a better answer to dev staleness.
- **Bootstrap on API startup.** Considered as belt-and-braces (the API pod reseeds the chain on start). Not needed — the cron tick is cheap and doesn't couple the API lifecycle to SB seed state. Can be added later without undoing this work.
- **Tighter cron cadence (e.g. `*/10 * * * *`).** Rejected — shrinks the worst-case recovery window but triples container-start rate. The chain losing its way should be an extenuating event, not routine.
