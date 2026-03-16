# Town Crier: Feature Plan

## Data Provider

[PlanIt](https://www.planit.org.uk) — Free, unauthenticated API providing UK planning application data across 417 Local Planning Authorities. Data is sourced from council planning portals (public information under statutory obligation). See [ADR 0006](adr/0006-planit-primary-data-provider.md) for selection rationale.

### Ingestion Model

Town Crier uses a **polling service** (not webhooks) to ingest data from PlanIt:

- A background service in Azure Container Apps polls PlanIt on a configurable interval (default: 15 minutes).
- Each poll queries `GET /api/applics/json` filtered by `recently_changed` to retrieve applications modified since the last successful poll.
- Results are diffed against Cosmos DB and upserted. The PlanIt application ID is the idempotency key.
- 15–30 minute polling latency is acceptable — planning consultation periods are measured in weeks.

### Available PlanIt Capabilities

| Capability | Endpoint | Notes |
|-----------|----------|-------|
| List/filter applications | `GET /api/applics/json` | Filter by authority, postcode, status, type, date range, `recently_changed` |
| Nearby spatial search | `GET /api/applics/json` | lat/lng + radius parameters |
| Get application by ID | `GET /api/applics/json?uid=` | PlanIt UID lookup |
| List authorities | `GET /api/authorities/json` | 417 LPAs with metadata |

### PlanIt Pricing

PlanIt is **free** — no API key, no subscription, no per-request charges. Rate limits are unpublished (429 responses); poll conservatively and back off on throttling.

### Not Available from PlanIt

- **Webhooks** — PlanIt has no push mechanism. Mitigated by the polling service.
- **Public comments / representations** — not exposed via the API. Future options: scrape council portals or deep link to council comment pages.
- **Published Terms of Service** — the API has no explicit ToS. Mitigated by conservative polling, aggressive caching, visible attribution, and planned outreach to the PlanIt maintainer.

### Attribution Requirements (Mandatory)

Town Crier must display data attribution in the app and on any public-facing surface:

1. **PlanIt** — "Planning data sourced from [PlanIt](https://www.planit.org.uk)"
2. **Crown Copyright (OGL)** — "Contains public sector information licensed under the Open Government Licence v3.0."
3. **Ordnance Survey** — "Contains OS data © Crown copyright and database right [year]."
4. **OpenStreetMap** — "Contains data from OpenStreetMap © OpenStreetMap contributors, ODbL."

---

## Authentication

**Provider:** [Auth0](https://auth0.com) (managed, not self-hosted)

- Username/password authentication from day one
- Passkeys and TOTP 2FA from day one (Auth0 configuration, not custom code)
- "Sign in with Apple" from day one (Auth0 social connection toggle)
- Free tier: 25,000 MAU (sufficient well beyond early growth)
- Native SDKs for both Swift (iOS) and .NET (API)

---

## Monetisation Model

### Pricing Tiers

| | Free | Personal (£1.99/mo) | Pro (£5.99/mo) |
|---|---|---|---|
| **Target user** | Curious resident | Homeowner / engaged local | Property professional / community organiser |
| Watch zones | 1 | 1 | Unlimited |
| Radius | 1km | 5km | 10km |
| Notifications | 5/week (new apps only) | Unlimited new + decisions | Unlimited all events |
| Data | Forward-only (no backfill) | Instant backfill of recent history | Instant backfill |
| Search | Browse cached list | Browse + filter by status/type | Full-text search |

### Cost Structure

**Baseline monthly costs (independent of user count):**

| Service | Estimated Cost |
|---------|---------------|
| PlanIt data provider | £0 |
| Cosmos DB Serverless | £5–15/mo |
| Azure Container Apps (consumption) | £5–10/mo |
| Auth0 | £0 (free tier) |
| Apple Developer Program | £79/year (~£6.60/mo) |
| **Total baseline** | **~£17–32/mo** |

**Marginal cost per tier:**

- **Free users**: ~£0 — served entirely from cached polling data, no additional PlanIt API calls
- **Personal users**: ~£0 — same cached data, backfill is a one-time PlanIt call per zone
- **Pro users**: Low — occasional PlanIt search calls for full-text search

**Break-even:** ~19 Personal subs or ~6 Pro subs.

### Zero-Cost Free Tier Strategy

The free tier avoids ongoing costs by:

1. **No backfill** — free users don't trigger PlanIt API calls on zone creation. Their map/list populates gradually from polling data.
2. **Shared application cache** — all polled data lands in Cosmos DB regardless of tier. Free users read from the same cache as paid users.
3. **Notification cap (5/week)** — limits compute, creates natural upgrade motivation ("12 new applications this week — upgrade to see all").
4. **No full-text search** — avoids PlanIt API passthrough calls.

### Upgrade Drivers

- **Free → Personal**: Notification cap. Once a user cares enough to hit 5/week, £1.99 is an impulse purchase. Larger radius is a secondary motivator.
- **Personal → Pro**: Multiple locations. Different use case entirely — landlords with scattered properties, parish councillors covering multiple wards, estate agents watching several postcodes.

---

## Feature Phases

### Phase 0 — Foundation

| # | Feature | Details |
|---|---------|---------|
| 0.1 | Project scaffolding | .NET 10 API skeleton (hexagonal architecture), iOS app shell (MVVM-C), Pulumi infra baseline, GitHub Actions CI/CD |
| 0.2 | Auth0 integration | Username/password registration and login, passkeys + TOTP 2FA, Sign in with Apple, JWT validation in API, Auth0 Swift SDK in iOS app |
| 0.3 | User profile & preferences | Cosmos DB user container storing postcode, watch zone, notification preferences, subscription tier |
| 0.4 | Cosmos DB data model | Containers: Users, WatchZones, Applications, Notifications. Partition strategy aligned with access patterns |

### Phase 1 — Data Pipeline

| # | Feature | Details |
|---|---------|---------|
| 1.1 | PlanIt polling service | Background service polling `GET /api/applics/json?recently_changed=` on configurable interval (default 15 min), with rate limit handling and exponential backoff on 429s |
| 1.2 | Application ingestion | Diff polled results against Cosmos DB, idempotent upsert by PlanIt application ID into Applications container |
| 1.3 | Watch zone matching | On ingestion, spatial match of application lat/lng against active user watch zones |
| 1.4 | Polling scope management | Configure polling queries filtered by authority for areas with active users, expanding coverage as user base grows |
| 1.5 | Backfill (paid users only) | On zone creation for Personal/Pro users, query PlanIt `GET /api/applics/json` with location parameters to seed recent applications |

### Phase 2 — Push Notifications (MVP)

| # | Feature | Details |
|---|---------|---------|
| 2.1 | APNs integration | Device token registration via API, push certificate management |
| 2.2 | Notification dispatch | Watch zone match → queue → push notification. Enforce weekly cap for free tier |
| 2.3 | Notification history | Stored in Cosmos DB, displayed as in-app feed |
| 2.4 | Notification preferences | Per-zone toggles: new applications (all tiers), status changes (Personal+), decision updates (Personal+) |

### Phase 3 — iOS App Experience

| # | Feature | Details |
|---|---------|---------|
| 3.1 | Map view | MapKit with application pins, colour-coded by status (Pending/Approved/Refused/Withdrawn) |
| 3.2 | Application detail | Address, description, status, dates, decision, link to council portal for public comments |
| 3.3 | Application list | Filterable list within watch zone. Free: browse only. Personal+: filter by status/type/date |
| 3.4 | Watch zone management | Add/edit/delete zones with postcode entry + radius picker + map preview. Free limited to 1 zone |
| 3.5 | Full-text search | Pro tier only. Pass-through to PlanIt search parameters |
| 3.6 | Deep links | Tap notification → opens relevant application detail screen |

### Phase 4 — Engagement & Retention

| # | Feature | Details |
|---|---------|---------|
| 4.1 | Status timeline | Visual lifecycle: Pending → Approved / Refused / Withdrawn with dates |
| 4.2 | Decision alerts | Targeted notification when a bookmarked application gets a decision |
| 4.3 | Saved applications | Bookmark specific applications for ongoing monitoring (all tiers) |
| 4.4 | Weekly digest | Push summary: "X new applications near you this week" (Personal+ tier) |
| 4.5 | Authority directory | Browse/search 417 authorities from cached `GET /api/authorities/json` data |

### Phase 5 — Growth

| # | Feature | Details |
|---|---------|---------|
| 5.1 | Dynamic polling optimisation | Adjust polling frequency and authority scope based on active user distribution to minimise PlanIt API load |
| 5.2 | Public comments (investigation) | Evaluate: deep links to council portals or selective scraping |
| 5.3 | Community groups | Shared watch zones for neighbourhood associations, parish councils |

---

## Cross-Cutting Concerns

| Area | Approach |
|------|----------|
| API cost management | Cache all polled data in Cosmos DB. Serve browse/list/map/detail from own data. Only call PlanIt for backfill and full-text search |
| Polling reliability | Idempotent upserts keyed on PlanIt application ID. Track last successful poll timestamp for change detection |
| Polling scaling | Start with authority-scoped polls for areas with active users. Broaden coverage as user density grows |
| Rate limit handling | Respect 429 responses, exponential backoff. PlanIt rate limits are unpublished — poll conservatively |
| Data freshness | Polling provides 15–30 minute latency, acceptable for planning applications with week-long consultation periods |
| Tier enforcement | API enforces limits (notification cap, zone count, radius, search access). iOS app shows upgrade prompts at limit boundaries |
