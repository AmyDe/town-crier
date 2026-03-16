# Town Crier: Feature Plan

## Data Provider

[Planwire.io](https://planwire.io) — REST API providing UK planning application data across 379 Local Planning Authorities. See [ADR 0003](adr/0003-external-data-provider.md) for selection rationale.

### Available Planwire Capabilities

| Capability | Endpoint | Notes |
|-----------|----------|-------|
| List/filter applications | `GET /v1/applications` | Filter by council, postcode, status, type, decision, date range, full-text `q` |
| Nearby spatial search | `GET /v1/applications/nearby` | lat/lng + radius (1–50km), PostGIS-backed |
| Get application by ID | `GET /v1/applications/:id` | UUID lookup |
| Get by council reference | `GET /v1/applications/ref/:council/:reference` | e.g. `ref/adu/AWDM%2F0158%2F25` |
| List councils | `GET /v1/councils` | 379 LPAs with metadata |
| Webhooks | `POST /v1/webhooks` | Events: `application.new`, `application.updated` |
| Webhook filters | — | `councilId`, `postcodePrefix`, `status`, `applicationType` |
| Webhook security | `X-PlanWire-Signature` | HMAC-SHA256 verification |

### Planwire Pricing

| Plan | Requests/day | Webhooks | Cost |
|------|-------------|----------|------|
| Free | 100 | None | £0 |
| Starter | 1,000 | 5 | £29/mo |
| Growth | 10,000 | Unlimited | £99/mo |
| Enterprise | Unlimited | Unlimited | £299/mo |

### Not Available from Planwire

- **Public comments / representations** — not exposed via the API. Future options: scrape council portals, deep link to council comment pages, or request from Planwire.
- **Full application data model** — not fully documented. A live API call is needed to discover all returned fields.

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
| Planwire Starter | £29/mo |
| Cosmos DB Serverless | £5–15/mo |
| Azure Container Apps (consumption) | £5–10/mo |
| **Total baseline** | **~£40–55/mo** |

**Marginal cost per tier:**

- **Free users**: ~£0 — served entirely from cached webhook data, no Planwire API calls
- **Personal users**: ~£0 — same cached data, backfill is a one-time Planwire call per zone
- **Pro users**: Low — occasional Planwire search calls for full-text search

**Break-even:** ~25 Personal subs or ~9 Pro subs.

### Zero-Cost Free Tier Strategy

The free tier avoids ongoing costs by:

1. **No backfill** — free users don't trigger Planwire API calls on zone creation. Their map/list populates gradually from incoming webhooks.
2. **Shared application cache** — all webhook data lands in Cosmos DB regardless of tier. Free users read from the same cache as paid users.
3. **Notification cap (5/week)** — limits compute, creates natural upgrade motivation ("12 new applications this week — upgrade to see all").
4. **No full-text search** — avoids Planwire API passthrough calls.

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
| 1.1 | Planwire webhook receiver | `POST /api/webhooks/planwire` with HMAC-SHA256 signature verification, idempotent upsert by application ID |
| 1.2 | Application ingestion | Parse `application.new` / `application.updated` payloads → upsert into Cosmos DB Applications container |
| 1.3 | Watch zone matching | On ingestion, spatial match of application lat/lng against active user watch zones |
| 1.4 | Webhook subscription management | Register Planwire webhooks filtered by postcode prefix for areas with active users |
| 1.5 | Backfill (paid users only) | On zone creation for Personal/Pro users, call `GET /v1/applications/nearby` to seed recent applications |

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
| 3.5 | Full-text search | Pro tier only. Pass-through to Planwire `q` parameter |
| 3.6 | Deep links | Tap notification → opens relevant application detail screen |

### Phase 4 — Engagement & Retention

| # | Feature | Details |
|---|---------|---------|
| 4.1 | Status timeline | Visual lifecycle: Pending → Approved / Refused / Withdrawn with dates |
| 4.2 | Decision alerts | Targeted notification when a bookmarked application gets a decision |
| 4.3 | Saved applications | Bookmark specific applications for ongoing monitoring (all tiers) |
| 4.4 | Weekly digest | Push summary: "X new applications near you this week" (Personal+ tier) |
| 4.5 | Council directory | Browse/search 379 councils from cached `GET /v1/councils` data |

### Phase 5 — Growth

| # | Feature | Details |
|---|---------|---------|
| 5.1 | Dynamic webhook management | Register/deregister Planwire webhooks based on active user distribution to optimise API costs |
| 5.2 | Public comments (investigation) | Evaluate: deep links to council portals, selective scraping, or Planwire feature request |
| 5.3 | Community groups | Shared watch zones for neighbourhood associations, parish councils |

---

## Cross-Cutting Concerns

| Area | Approach |
|------|----------|
| API cost management | Cache all webhook data in Cosmos DB. Serve browse/list/map/detail from own data. Only call Planwire for backfill and full-text search |
| Webhook reliability | Idempotent upserts keyed on Planwire application ID. Monitor delivery logs via `GET /v1/webhooks/:id/deliveries` |
| Webhook scaling | Start with postcode-prefix-filtered webhooks. Consolidate into broader filters as user density grows in an area |
| Rate limit handling | Respect 429 responses, exponential backoff, monitor daily quota. Limits reset at midnight UTC |
| Data freshness | Webhooks provide near-real-time updates. Periodic `nearby` poll as safety net for missed deliveries (paid tiers only) |
| Tier enforcement | API enforces limits (notification cap, zone count, radius, search access). iOS app shows upgrade prompts at limit boundaries |
