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

PlanIt is a free service run by one person and our only planning-data provider (ADR 0006). Not hammering it is non-negotiable. PlanIt's operator publishes usage guidance for API users, including a daily request cap (see "Operator guidance" below). ADR 0041 and ADR 0042 cite a `~1,500 requests/day` figure. That is history, not a live limit.

**Two states are described below.** "Rebuilt poller" is the design in ADR 0049 (GH#1178), built in a series of PRs and live only after the prod release tag. "Current prod" is the lane-based poller until that tag. Where a section differs between them it says which.

**The rebuilt poller is built to meet the operator's guidance.** One call is one request, with no internal retries. A `planit_call` table is the only pacer state. Before each request the pacer applies the backoff from the latest call (429: `Retry-After` up to 3h, or 15m if absent; 403: 24h; timeout or 5xx: 30m), checks the daily cap, checks the hourly cap (at most 15 calls in any rolling hour, see fact 24), and waits at least 60 seconds after the previous call. When the hourly cap applies, the run waits for the oldest call of the hour to age out, or stops with `hourly_cap` if that is after the run budget. Every attempt counts against the cap, including failures. The budget day runs 18:00 to 18:00 Europe/London. Prod caps at 240 calls a day and dev at 60, so together they stay within 300 (see "Settled decisions").

**Current prod (until the release tag) has brakes but does not meet the guidance.** It honours `Retry-After` (a 429 is never retried internally; it ends the cycle and the scheduler reschedules), backs off for 2h after a timeout, caps attempts at 4, and sleeps 2 seconds before every attempt including retries (`api-go/internal/planit/client.go`). It does not follow the published pacing, daily cap, time window or User-Agent rules. See "Current prod gaps".

**A local session is the biggest risk**: a `curl` loop, a throwaway script, a "let me just test this quickly" harness. It has none of those brakes and no review. Binding on any local or agent session calling PlanIt by hand:

- **Never more than 10 requests total in a session.**
- **Never more than one request per 60 seconds.**
- **Never write a loop that hits PlanIt without an explicit sleep of 60s or more between iterations.**
- **Never send a custom User-Agent.** No app name, no `towncrierapp.uk`, no email. Local traffic must never be affiliated with Town Crier, so that a block on local debugging cannot also block the prod poller, and the reverse.
- **Never put a personal email in any request**, header or payload, and never in a brief or prompt for an agent that calls PlanIt.

No exceptions for "it's only a few more", for batching, or for running in parallel. If a task looks like it needs more than 10 calls, stop and ask. Prefer the `httptest`-backed doubles in `api-go/internal/planit/*_test.go` over live calls. Every brief that lets a subagent call PlanIt must repeat these rules.

## User-Agent

- **Prod and dev pollers (rebuilt poller):** `TownCrier/<version> (+https://towncrierapp.uk; support@towncrierapp.uk)`, sent on every request from both environments. The contact is always `support@towncrierapp.uk`, never a personal address. Current prod sends none.
- **Local and agent sessions:** no custom User-Agent (see above).

## PlanIt facts

### The service

1. PlanIt (planit.org.uk) is our only data source. It is free and run by one person as a "retirement hobby" on a "best efforts" basis, with no service standards. The FAQ says "it would not be wise to base any commercial services on the API".
2. There is no other feed, export or push mechanism. The search API is all we have. `georss` is only another output format of `/api/applics`, not a push feed, and the same limits apply. PlanIt does not make the full dataset available, and there are "no commercial terms". The paid client list for the scraper software is "currently full".
3. PlanIt publishes guidance, not a hard limit (see "Operator guidance" below). The API page says rate limits exist but does not state them. When it decides we are sending too much it returns 429 with `Retry-After`, which can be several hours.
4. Under load PlanIt usually gets slower and times out before it starts returning 429s. The timeout rate is the better load signal.
5. PlanIt has full outages (for example 2026-06-18: 89 of 89 calls failed).

### Operator guidance

From the PlanIt FAQ (https://www.planit.org.uk/faq/, read 2026-09-28), for anyone keeping a database up to date with the API:

- "Run overnight, 18:00-06:00, so as not to compete with daytime users"
- "Leave a minimum 60 seconds between requests, with adaptive backoff that honours the Retry-After header"
- "Try to limit yourself to a daily request cap of 300"
- "Place an identifier string, including your email, in the User-Agent field"

The FAQ also says "you can safely make one request to the /api/applics endpoint every minute", and "I will block requests without a valid user agent or those that exceed a reasonable number per day". Donations are welcome. The contact is andrew@planit.org.uk.

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

### Further API facts (probed 2026-09-28, GH#1178)

20. `start_date` with `end_date`, and `decided_start` with `decided_end`, are inclusive and exact for a single day: a probe of one decided day returned 1,812 records, all with that `decided_date`. `different_end` and `changed_*` filters also exist and are inclusive.
21. A request with a bad or missing User-Agent can get 403.
22. Deep offsets time out. Timeouts in prod clustered at `index >= 30000`. A single day window is at most about 10 pages, which keeps offsets small.
23. `sort=area_id,uid` is accepted and stable, but PlanIt has no filter that starts after a key, so we cannot page by key.

### Observed limits (prod, 2026-10-05/06)

24. **PlanIt answers 429 after about 20 calls in one hour, even at 60 s spacing.** On the first night of the rebuilt poller, every hourly run got a 429 after 26 to 30 calls. `Retry-After` was 2,318 to 2,499 s, and every backoff ended at hh:07:59, so PlanIt appears to count calls in fixed hours that start at about :08. Each run made 8 calls before :08 and then 18 to 22 more before the 429. The old poller also stopped after 18 to 20 calls per hour on 2026-10-05. The operator's "one request every minute" (see "Operator guidance") is therefore not safe as a sustained rate. The pacer caps calls at 15 in any rolling hour to stay well below the observed limit.

## Requirements

1. **Same-day alerting**, with a 14-day limit for late finds (see the goal above). This applies to new applications and to decisions, each on its own date.
2. **Complete coverage.** Every application inside a watch zone must be in our database and visible in the app, whether or not it triggers a notification.
3. **No churn alerts.** A record that only looks changed because PlanIt re-indexed it must not alert. The only changes that alert are a new application and a decision. Before ADR 0041 this was about 450 false alerts a day for one user.
4. **At most one push per watch zone per poll cycle.**
5. **National coverage.** Watch zones are circles up to 10 km anywhere in the UK, so polling must cover the whole country.
6. **Polite behaviour** as described under the hard call limits and the operator guidance.
7. **Safe rollout.** We have paying customers, so polling changes need a soak and a rollback path.

Entitlements (who gets instant push, email or only the weekly digest) are decided at dispatch time, not by polling. Polling must ingest every record regardless of tier.

## Settled decisions

- **Do not poll per authority.** We ran per-authority polling. It was much more complicated, it was not required, and we will not revisit it. Poll nationally. The old per-authority code (`PollPlanItHandler` in `api-go/internal/polling/handler.go`) is unwired and must not be revived. The single dev `auth=<area_id>` filter is a fixed test subset, not per-authority polling.
- **Daily call budget: 300 a day, decided (ADR 0049).** This is the operator's guidance. Prod and dev are one client to the operator because they send the same User-Agent, so their caps together must not go above 300: prod 240, dev 60. This replaces the earlier "no daily call budget" decision. Applies to the rebuilt poller.
- **Poll by date window, not by change signal (ADR 0049).** The rebuilt poller finds records by their `start_date` or `decided_date`, not by `last_different`. It re-reads every alert-band day window each night, uses a count probe when a window's `total` has not changed, and forces a full read every 7 days.
- **At most 15 calls in any rolling hour (tc-oh5p4).** This is from fact 24, not from the operator's guidance, and it applies to each environment's pacer. It is more restrictive than 60 s spacing alone. Do not raise it without new evidence that PlanIt accepts more. 15 an hour for the 12 night hours is 180 calls, which equals the night share of the prod cap (240 minus the 60 day allowance).
- **Only one environment polls at a time (GH#1191).** Each environment has its own pacer, so prod and dev together could send two requests within 60 seconds. A polling switch per environment (`poll_control`, default from `POLLING_ENABLED_DEFAULT`: prod on, dev off) makes the pacer refuse every request while it is off. A request already scheduled when the switch goes off still completes, so it can be sent up to 90 seconds later (60 s spacing plus the 30 s timeout). Move polling with `tc polling-switch --to <dev|prod|off>`: it turns the other environment off first, waits until no request from it can still be sent less than 60 seconds before the target's first request, then turns the target on and verifies each step. `tc polling-status` shows both. Switching to dev stops prod alerts until you switch back. The old prod poller (before the release tag) has no switch.
- **Dev polls one authority and never reads prod.** Dev runs the same poller with `auth=<area_id>` for one authority. A dev oracle checks the day windows against an independent wide read, and is the gate for the prod release tag.

## Useful numbers

- Only about 5 to 12 recent applications a day land inside any watch zone nationally. The set that matters is small, but we cannot tell which records matter until we have their location.
- Both pollers run an hourly job.
- Current prod made 500 to 760 PlanIt requests a day from 2026-09-23 to 2026-09-27, and about half of them between 06:00 and 18:00 UTC (App Insights `AppDependencies`).
- One weekday `start_date` day is about 2,000 to 2,300 records (7 to 8 pages), and one `decided_date` day about 1,800 (7 pages). A full re-read of all 30 alert-band windows is about 150 requests, which is why the count probe exists.
- Rebuilt prod is estimated at 175 to 225 requests a budget day against a cap of 240. The `planit_call` log gives the real numbers in the first week.
- Per-record `id_match` lookups (fact 12) cost one request each, so the rebuilt poller uses none.

## Known limits of the rebuilt poller

- **Null `start_date`.** A record with no `start_date` is in no start window, and the day delta is masked by `start_date`. GH#1178 pre-work P1 measures how many exist. A probe found none with a null `decided_date`.
- **Late by more than 14 days.** A record first seen more than 14 days after its date is stored (the coverage band reads ages 15 to 90 at least every 7 days) but does not alert.
- **Records PlanIt does not have** (fact 19) cannot be found.
- **The day delta slots are outside the operator's 18:00 to 06:00 guidance.** About 50 calls a day are in the daytime. We have not asked the operator whether that is acceptable, and ADR 0006 planned to contact him after the MVP with no record that we did.
- **Dev proves the subset only.** The oracle checks one authority, not the country.

## Current prod gaps (until the rebuilt poller ships)

- Lane A stops at the first record not newer than its `last_different` watermark, so late records (fact 17) are skipped permanently. Lane E (ADR 0047) was built to catch them but is not enabled in prod.
- The code does not use the 14-day limit. Lanes A and B can alert on anything inside their 90-day masks, and Lanes C and E gate alerts at 30 days.
- It does not meet the operator guidance:

  | Operator guidance | Current prod |
  |---|---|
  | 60 s between requests | `PLANIT_THROTTLE_DELAY_SECONDS=2` (`infra/environment.go`) |
  | User-Agent with an email | None set in `api-go/internal/planit/client.go`, so Go sends `Go-http-client/1.1` |
  | About 300 requests a day | 500 to 760 a day |
  | 18:00 to 06:00 only | About half the requests are in daytime |

  The FAQ says requests without a valid User-Agent, or too many a day, are blocked. This is a probable cause of some 429s and timeouts, but that is not proven.

## Further reading

ADR 0049 (day-window polling rebuild, the current design), ADR 0006 (PlanIt as provider). Superseded by ADR 0049 and kept for history: ADR 0024 (Service Bus polling), ADR 0038 (dev-seed), ADR 0041 (churn-masked national delta axis), ADR 0042 (historical backfill, Lane D), ADR 0043, ADR 0044 (resumable checkpointed national flow, Lane C), ADR 0047 (Lane E recent-window sweep). The full design and acceptance criteria are in GH#1178.
