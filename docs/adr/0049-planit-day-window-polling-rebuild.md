# 0049. Rebuild PlanIt polling on day windows and an event outbox

Date: 2026-09-29

## Status

Accepted

Supersedes [0024](0024-service-bus-only-polling.md) (the polling trigger and the bootstrap chain), [0038](0038-dev-seed-least-privilege-prod-read.md), [0041](0041-poll-planit-on-a-churn-masked-delta-axis.md), [0042](0042-historical-backward-backfill-lane.md), [0043](0043-keep-lane-d-out-of-lane-a-mask-band.md), [0044](0044-resumable-checkpointed-national-poll-flow.md) and [0047](0047-lane-e-recent-window-sweep.md). Design detail and acceptance criteria live in GitHub issue #1178.

## Context

Every earlier polling design found records through a change signal: `last_different`, `different=N`, a watermark or a cursor. A record that did not give that signal at the right time was never seen, and nothing looked for it again. A late record keeps its older `last_different`, so Lane A walked past it permanently (POLLING.md fact 17). In Kingston on 2026-09-27, 37 of 148 recent records appeared late and none reached prod. Lanes C, D and E each patched a hole left by the lane before, and each patch added state, config and alerts.

PlanIt has no cursor and no filter that starts after a key, so we cannot page by key. We must re-read a bounded set. The 14-day alert limit (POLLING.md requirement 1) gives the bound.

We were also not a polite client. Prod sent no User-Agent, waited 2 seconds between requests, made 500 to 760 requests a day and sent about half of them in the daytime. The Service Bus trigger made compliance impossible: its 5 minute lock allowed a 240 second cycle, which is four requests at 60 second spacing. Dev had no poller and relied on a job that copied applications out of prod through a read-only identity.

## Decision

Replace the five lanes, the Service Bus trigger, the per-lane notification gates and the dev-seed job with one design.

1. **Day windows.** Records are found by what they are, not by when they changed. Every `start_date` day and every `decided_date` day in the alert band (the last 14 days) is re-read every night. A coverage band (ages 15 to 90) is re-read at least every 7 days and stores records without alerting. Nothing older than 90 days is read.
2. **Count probe and forced full read.** A window whose PlanIt `total` has not changed since its last complete read costs one request. A full read is forced every 7 days, which closes the case where one record is added and one removed on the same night while the record can still alert.
3. **Atomic windows with run-time checks.** A window read is complete only when every page is read in one run. An integrity check (every record has its axis date in the window) and a completeness check (distinct records read are at least `total`) run on every read.
4. **Day delta passes for latency only.** A few delta reads in the daytime bring new records in sooner. Correctness never depends on them. A delta cross-check counts any record a delta saw that a later full read did not return, as a permanent self-check.
5. **Event outbox.** Ingest writes an event only for a first-seen application (`new_application`) and for a change into a decision state (`decision`). One dispatcher inside the poll job applies the 14-day limit, quiet hours and a surge check, then hands off to the existing notification fan-out. Events from a surge are marked stale, not held.
6. **Paced hourly poll job.** A scheduled job, `WORKER_MODE=poll`, replaces the Service Bus job and its bootstrap. A `planit_call` table is the only pacer state: 60 seconds between requests, a daily cap, and backoff from the latest call (429 honours `Retry-After` up to 3 hours, 403 backs off 24 hours, timeouts and 5xx back off 30 minutes). The budget day runs from 18:00 to 18:00 Europe/London so one night never spans two budgets. The cap is 240 in prod and 60 in dev, so the two together stay within the operator's 300 a day. Both environments send the same User-Agent with `support@towncrierapp.uk`.
7. **Health from the database.** Each run computes `ok`, `degraded` or `critical` in Go and sets it on the `PlanIt poll run` span. Three shared alerts (`alert-planit-poll-critical-shared`, `alert-planit-poll-degraded-shared`, `alert-planit-poll-heartbeat-shared`) replace the lane alerts.
8. **Dev polls one authority, and never reads prod.** Dev runs the same poller with one extra filter, `auth=<area_id>`. A permanent oracle in dev makes a second, independent wide read of the same subset and records any record the day windows missed in `poll_oracle_diff`. The oracle is the gate for the prod tag: at least 3 nights with no `miss`, no failed integrity or completeness check and no delta cross-check misses.
9. **Big-bang cutover.** No parallel run in prod. The rebuild merges to `main` and deploys to dev, the dev trial runs until the gate passes, then one `v*` release ships and all old polling logic is deleted in the same release. Rollback is revert plus retag. The old state tables (`poll_state`, `backfill_state`, `recent_sweep_state`) stay for 14 days after the prod release so the old code still finds them, then a follow-up drops them.

Migration 0031 creates `planit_call`, `poll_window`, `delta_seen`, `application_event`, `poll_window_member` and `poll_oracle_diff`. Migration 0032 adds `poll_event`, which slice 6 introduced beyond GH#1178 to hold durable 24 hour health facts (for example a 403 or a short window) so that health survives across runs and lease holders.

## Consequences

- A late record is found by the next night's window read as long as it is within 14 days of its date, and it alerts. Nothing depends on a change field.
- A run compares each record's axis date and each window's total with what it should be, so a PlanIt change of behaviour shows up as a health reason, not as silent loss.
- Prod meets the operator's guidance on spacing, User-Agent and the daily cap. The day delta passes (about 50 calls a day) are the one part outside the operator's 18:00 to 06:00 window, and are still to be confirmed with him.
- The Service Bus queue, its bootstrap job, the dev-seed job and its reader identity, and the lane state and config are deleted. Removing the dev-seed role from prod Postgres is a manual follow-up.
- Records with a null `start_date` are in no start window, and PlanIt records that PlanIt itself lacks cannot be found by any design. Both are accepted limits.
- The first night after cutover reads every alert-band window and can send a burst of real alerts for records the old lanes missed. Pushes are coalesced to one per watch zone per run, and the surge threshold guards against a re-key.
- Dev covers one authority. The oracle is affordable only because of that, and it proves the window reads on that subset, not nationally.
- Correctness now rests on three run-time-checked facts about PlanIt (exact date filters, complete window reads and nightly coverage of the band), instead of an implicit assumption that change signals never lie.
