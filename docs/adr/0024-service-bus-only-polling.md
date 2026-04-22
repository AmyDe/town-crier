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

## Amendments

### 2026-04-22 — receive-and-delete + publish-after-consume

Observed on the first post-deploy day: the `poll-triggers` queue accumulated **14 active messages** over roughly one hour, causing KEDA to drain the backlog at its 30-second polling cadence and hammer PlanIt with ~30 zero-work 429s over ~50 minutes. The scheduled (future-dated) messages were sitting correctly in the `scheduledMessageCount`, but they sat behind a growing pile of already-active duplicates.

Root cause: the original **publish-before-ack** ordering produces a duplicate whenever `CompleteAsync` fails or doesn't complete in time. Sequence:

1. Orchestrator publishes the next-run message (scheduled for `now + retryAfter`).
2. `CompleteAsync` fails — container killed, REST timeout, lock expired mid-call, or any transient SB error.
3. The Service Bus PeekLock expires (bounded by the 5-minute Basic-tier `LockDuration` cap — see `asb-lockduration-capped-at-5m`).
4. The unacked message redelivers to a new replica. That replica processes it, publishes *another* next-run message, and acks.
5. Net: the queue gains one message per occurrence. Over many cycles the backlog compounds.

The original "publish-before-ack is load-bearing for crash safety" framing anticipated the wrong failure mode: at-least-once delivery via redelivery was supposed to make the chain self-healing, but for this workload redelivery is actively harmful — the handler already hit PlanIt, and re-running it just burns quota while duplicating the outgoing next-run message.

#### Amended decision

1. **Orchestrator uses Service Bus receive-and-delete mode**, not peek-lock. The message is destructively consumed on receive. There is no lock, no `Complete`, no `Abandon`.
2. **Ordering is consume → process → publish.** Publish happens after the handler has fully run. If anything fails between receive and publish, the chain pauses until the next safety-net tick.
3. **`IPollTriggerQueue.CompleteAsync` and `AbandonAsync` are removed.** The LeaseHeld branch in the orchestrator exits without settling — the message is already gone, and the lease holder is responsible for the next publish.
4. **Safety-net probe uses the Service Bus management API**, reading `activeMessageCount + scheduledMessageCount` on the queue. Seed only when both are zero. The previous PeekLock-based probe was destructive under receive-and-delete and blind to scheduled messages (silently causing duplicate bootstrap publishes even under the old design).
5. **`POLLING_HANDLER_BUDGET_SECONDS` can relax.** It existed solely to bail before the 5-minute PeekLock cap. With no lock, the only bound is `replicaTimeout` (600 s).

#### Failure modes under the amendment

| Failure point | Outcome |
|---------------|---------|
| Crash before/during processing | Message gone, no next published → safety-net recovers within ≤30 minutes |
| Processing succeeds, publish fails | Same as above |
| Receive succeeds, process + publish succeed | Normal chain continues |

All failure paths converge on the same safety-net recovery path. At-most-once delivery is accepted as the trade-off; the work itself (polling PlanIt) is idempotent over the polling cursor and is intended to be retried on a timer in any case.

#### Consequences of the amendment

**Easier**

- Active queue depth stays bounded to 0–1 under normal operation. Observability improves: `activeMessageCount > 1` is now a clear bug signal.
- Code simpler: no `Complete`/`Abandon` paths, no lock-URL threading, no LeaseHeld-Abandon branch, no 4-minute handler budget tied to a 5-minute lock cap.
- Safety-net probe correctly sees scheduled messages for the first time — pre-existing TOCTOU window is closed.

**Harder**

- Worst-case polling stall on a single publish-side failure is now ≤30 minutes (the safety-net cadence) rather than ≤5 minutes (the PeekLock redelivery). Acceptable for this workload; would not be acceptable for a user-facing path.
- Safety-net bootstrap is the sole recovery mechanism and must stay healthy. Its own failures (management-API RBAC, SB outage, cron misfire) now have no backstop. Mitigated by the 30-minute cadence being cheap enough to keep.
- Worker identity needs a management-plane read role on the Service Bus namespace (in addition to the existing data-plane Data Owner role). Least-privilege TBD during implementation (likely `Reader` scoped to the queue).

See bead **tc-gpof** for the implementation and **tc-ku5u** for the original observation.
