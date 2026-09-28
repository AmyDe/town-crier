# POLLING.md

Facts about PlanIt and about what Town Crier's polling must achieve. Read this before you design, change, debug, review or discuss anything that touches PlanIt or polling (`api-go/internal/polling`, `api-go/internal/planit`, the poll worker, its alerts and its config).

This file holds facts and requirements, not a design. Design rationale lives in the ADRs listed at the end. When a fact here changes, update this file in the same PR.

## The goal

We alert users about two different events. They are separate, and each one alerts on its own:

| Event | What it means | PlanIt signal | Date that sets its age |
|---|---|---|---|
| **New application** | The council has received and registered a planning application. | A record we have never seen before. | `start_date` |
| **Decision** | The council has decided an application that already exists. | `app_state` changes to a decision state (`Permitted`, `Conditions`, `Rejected`, `Appealed`). | `decided_date` |

One application can produce both alerts: one when it is created, and one later when it is decided. If we first see an application after it is already decided, both events are new to us, and each follows the rules below on its own date.

**Target:** a user gets the alert on the same day that the event becomes available on PlanIt. "Available" means the moment the record (or its new state) is searchable on PlanIt, not the date in any of its fields. An event that appears late (see PlanIt facts 16 to 18) still has to alert on the day it appears.

**Limit:** same-day is not always possible (PlanIt outages, 429s, late records we find later). We still alert when we find the event late, as long as the event is **no more than 14 days old**, measured from its own date in the table above. An event older than 14 days is ingested and shown in the app, but it does not alert.

## Hard call limits for local sessions

PlanIt is a free service run by one person and our only planning-data provider (ADR 0006). Not hammering it is non-negotiable, but the rule is about behaviour, not a daily quota. There is no daily call budget: do not introduce one, enforce one, or raise findings based on one. ADR 0041 and ADR 0042 cite a `~1,500 requests/day` figure. That is history, not a live limit.

**The deployed poller is not the risk.** It honours `Retry-After` (a 429 is never retried internally; it ends the cycle and the scheduler reschedules), backs off for 2h after a timeout, caps attempts at 4, and sleeps 2s before every attempt including retries. That is what "polite" means here, and `api-go/internal/planit/client.go` enforces it.

**A local session is the risk**: a `curl` loop, a throwaway script, a "let me just test this quickly" harness. It has none of those brakes and no review. Binding on any local or agent session calling PlanIt by hand:

- **Never more than 10 requests total in a session.**
- **Never more than one request per 60 seconds.**
- **Never write a loop that hits PlanIt without an explicit sleep of 60s or more between iterations.**

No exceptions for "it's only a few more", for batching, or for running in parallel. If a task looks like it needs more than 10 calls, stop and ask. Prefer the `httptest`-backed doubles in `api-go/internal/planit/*_test.go` over live calls.

## PlanIt facts

### The service

1. PlanIt (planit.org.uk) is our only data source. It is free and run by one person.
2. There is no other feed, export or push mechanism. The search API is all we have.
3. PlanIt publishes no rate limit. When it decides we are sending too much it returns 429 with `Retry-After`, which can be several hours.
4. Under load PlanIt usually gets slower and times out before it starts returning 429s. The timeout rate is the better load signal.
5. PlanIt has full outages (for example 2026-06-18: 89 of 89 calls failed).

### The query API

6. One search endpoint: `/api/applics/json`. Filters we use: `auth`, `start_date` / `end_date`, `decided_start`, `different_start`, `different=N`, `id_match`.
7. A page is at most 300 records (`pg_sz=300`), and PlanIt asks callers not to raise it. Paging is by `index=`.
8. The cost of a query is almost all in the first page, where PlanIt counts and sorts the whole result set. Later pages of the same query are cheap. Measured 2026-09-07: a 30-day `start_date` window took 2.8s at `index=0` and 0.2s at `index=3000`. So narrow windows are cheaper end to end than wide ones.
9. Wide queries hit a cliff. A 90-day `start_date` window took 17s. A `different_start` seven weeks in the past took more than 45s and then returned 400.
10. `different_start` and `different=N` are whole-day filters, not timestamps.
11. A sort field must also be in `select=`, or PlanIt returns 400.
12. `id_match` takes exactly one uid. A comma-separated list silently returns zero records (verified 2026-09-08). Looking records up one by one is one request per record, always.
13. `uid` is unique only within one authority. Different councils mint the same reference (for example Bassetlaw and Croydon). The identity of an application is `(uid, area_id)`.

### The data

14. About 131,800 records nationally have a `start_date` in the last 90 days (2026-09-07).
15. `last_different` is when PlanIt's scraper saw a change. PlanIt also bumps it when it re-indexes a record with no real change, so the raw `last_different` axis is dominated by churn on old applications.
16. Records do not become searchable at their `last_different` time. Most appear in one batch at about 07:00 UTC. Others appear hours or more than a day later.
17. **A late record keeps its older `last_different`.** `last_changed` approximates when it became visible. Assume late records happen in every authority. Measured in Kingston on 2026-09-27: 37 of 148 records with a `start_date` since 2026-08-15 appeared 14 to 28 hours after their `last_different`, and none of the 37 reached prod.
18. Councils can back-date `start_date`, so a new record can appear with a `start_date` that is days or weeks old. Under the 14-day limit, a new application that first appears more than 14 days after its `start_date` does not alert.
19. PlanIt is not a mirror of each council. Its scrapers can fall behind or miss records, and we cannot correct that.

## Requirements

1. **Same-day alerting**, with a 14-day limit for late finds (see the goal above). This applies to new applications and to decisions, each on its own date.
2. **Complete coverage.** Every application inside a watch zone must be in our database and visible in the app, whether or not it triggers a notification.
3. **No churn alerts.** A record that only looks changed because PlanIt re-indexed it must not alert. The only changes that alert are a new application and a decision. Before ADR 0041 this was about 450 false alerts a day for one user.
4. **At most one push per watch zone per poll cycle.**
5. **National coverage.** Watch zones are circles up to 10 km anywhere in the UK, so polling must cover the whole country.
6. **Polite behaviour** as described under the hard call limits.
7. **Safe rollout.** We have paying customers, so polling changes need a soak and a rollback path.

Entitlements (who gets instant push, email or only the weekly digest) are decided at dispatch time, not by polling. Polling must ingest every record regardless of tier.

## Settled decisions

- **Do not poll per authority.** We ran per-authority polling. It was much more complicated, it was not required, and we will not revisit it. Poll nationally. The old per-authority code (`PollPlanItHandler` in `api-go/internal/polling/handler.go`) is unwired and must not be revived.
- **No daily call budget** (see above).

## Useful numbers

- Only about 5 to 12 recent applications a day land inside any watch zone nationally. The set that matters is small, but we cannot tell which records matter until we have their location.
- Prod polls about once an hour.

## Known gaps (as of 2026-09-28)

- Lane A stops at the first record not newer than its `last_different` watermark, so late records (fact 17) are skipped permanently. Nothing currently enabled catches them. Lane E (ADR 0047) was built to, but is not enabled in prod.
- The code does not use the 14-day limit. Lanes A and B can alert on anything inside their 90-day masks (`POLLING_LANE_A_MASK_DAYS`, `POLLING_LANE_B_MASK_DAYS`), and Lanes C and E gate alerts at 30 days (`POLLING_LANE_C_NOTIFY_RECENCY_DAYS`, `POLLING_LANE_E_NOTIFY_RECENCY_DAYS`).
- `docs/product-overview.md` is out of date on polling. It says 15 minutes and Cosmos DB change feed. Trust this file.

## Further reading

ADR 0006 (PlanIt as provider), ADR 0041 (churn-masked national delta axis), ADR 0042 (historical backfill, Lane D), ADR 0044 (resumable checkpointed national flow, Lane C), ADR 0047 (Lane E recent-window sweep).
