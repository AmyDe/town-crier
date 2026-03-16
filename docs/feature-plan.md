# Town Crier: Feature Plan

## Data Provider

[PlanIt](https://www.planit.org.uk) — Free, unauthenticated API providing UK planning application data across 417 Local Planning Authorities. Data is sourced from council planning portals (public information under statutory obligation). See [ADR 0006](adr/0006-planit-primary-data-provider.md) for selection rationale.

### Ingestion Model

Town Crier uses a **polling service** (not webhooks) to ingest data from PlanIt:

- A background service in Azure Container Apps polls PlanIt on a configurable interval (default: 15 minutes).
- Each poll queries `GET /api/applics/json` filtered by `different_start` to retrieve applications whose content actually changed since the last successful poll (not `changed_start`, which updates on every scrape cycle even when content is unchanged — see [ADR 0006](adr/0006-planit-primary-data-provider.md)).
- Results are diffed against Cosmos DB and upserted. The PlanIt `name` field (`{area_name}/{uid}`) is the globally unique idempotency key.
- 15–30 minute polling latency is acceptable — planning consultation periods are measured in weeks.

### Available PlanIt Capabilities

| Capability | Endpoint | Notes |
|-----------|----------|-------|
| List/filter applications | `GET /api/applics/json` | Filter by authority, postcode, status, type, date range, `different_start`/`changed_start` |
| Nearby spatial search | `GET /api/applics/json` | lat/lng + radius parameters |
| Get application by ID | `GET /planapplic/{id}/json` | PlanIt UID lookup (also available via `GET /api/applics/json?uid=`) |
| List authorities | `GET /api/areas/json` | 417 LPAs with metadata |

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
| Notifications | 5/month (new apps only) | Unlimited (new, status changes, decisions) | Unlimited (all events) |
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
3. **Notification cap (5/month)** — limits compute, creates natural upgrade motivation ("You've reached your monthly limit — upgrade to see all").
4. **No full-text search** — avoids PlanIt API passthrough calls.

### Upgrade Drivers

- **Free → Personal**: Notification cap. Once a user cares enough to hit 5/month, £1.99 is an impulse purchase. Larger radius is a secondary motivator.
- **Personal → Pro**: Multiple locations. Different use case entirely — landlords with scattered properties, parish councillors covering multiple wards, estate agents watching several postcodes.

---

## Feature Phases

### Phase 0 — Foundation

| # | Feature | Details |
|---|---------|---------|
| 0.1 | .NET API scaffolding | .NET 10 API skeleton with hexagonal architecture (domain, application, infrastructure, web layers), health endpoint, Dockerfile (Alpine, Native AOT) |
| 0.2 | iOS app scaffolding | iOS app shell with MVVM-C architecture, SPM package structure (`town-crier-domain`, `town-crier-data`, `town-crier-presentation`) |
| 0.3 | Infrastructure baseline | Pulumi stacks provisioning core Azure resources (see [Infrastructure](#infrastructure) below) |
| 0.4 | CI/CD pipelines | GitHub Actions workflows for API, iOS, and infrastructure (see [CI/CD](#cicd) below) |
| 0.5 | Auth0 integration | Username/password registration and login, passkeys + TOTP 2FA, Sign in with Apple, JWT validation in API, Auth0 Swift SDK in iOS app |
| 0.6 | User profile & preferences | Cosmos DB user container storing postcode, watch zone, notification preferences, subscription tier |
| 0.7 | Cosmos DB data model | Containers: Users, WatchZones, Applications, Notifications. Partition strategy aligned with access patterns. Spatial index policy on Applications container for ST_DISTANCE queries against application lat/lng |
| 0.8 | App Store Connect setup | Apple Developer Program enrolment, App Store Connect app record, provisioning profiles (development + distribution), TestFlight configuration, bundle identifiers. App Store metadata: description, keywords, category (Utilities or Navigation), age rating questionnaire, screenshots for all required device sizes (6.7", 6.5", 5.5"), app preview video (optional). Required before iOS CI/CD pipeline can archive or distribute |
| 0.9 | Structured logging & observability | Structured JSON logging via `ILogger` with correlation IDs. Application-level health metrics (request latency, error rates) surfaced in Log Analytics. Crash reporting in iOS (consider lightweight solution — e.g., Firebase Crashlytics or native `MetricKit` crash diagnostics). Lays groundwork for polling health monitoring (1.7) |
| 0.10 | API versioning | URL-path versioning (`/v1/`) from day one. iOS clients in the field cannot be force-updated, so breaking changes require a new version segment. Old versions supported for a minimum deprecation window |
| 0.11 | StoreKit 2 & subscription management | iOS: StoreKit 2 integration for Personal and Pro auto-renewable subscriptions, transaction listener, entitlement resolution, "Restore Purchases" button (App Store requirement), and subscription disclosure UI showing auto-renewal terms, price, and cancellation instructions before purchase (App Store requirement). Optional free trial period (e.g., 7-day Personal trial) to drive conversion. API: Apple App Store Server Notifications v2 endpoint for real-time subscription lifecycle events (renewal, expiry, refund, grace period, billing retry). Cosmos DB stores canonical subscription state per user, synced from server notifications. Receipt validation via App Store Server API (not on-device). User-facing billing state: clear messaging during grace periods and billing retry windows |
| 0.12 | Privacy & account management | GDPR compliance: privacy policy (in-app and App Store listing), Terms of Service / EULA (custom, covering subscription terms and data usage — Apple provides a default EULA but subscription apps benefit from a custom one), data export endpoint (`GET /v1/me/data`), account deletion endpoint (`DELETE /v1/me`) that purges user data, watch zones, and device tokens. Account deletion is also an App Store Review requirement. Consent capture for notification permissions and data processing |
| 0.13 | API rate limiting | Per-user request throttling on the Town Crier API using sliding window middleware. Prevents abuse from free-tier clients and protects downstream resources. Separate rate limit tiers aligned with subscription tiers |
| 0.14 | App Store review preparation | Demo/review account with pre-seeded data so Apple reviewers (based in the US) can exercise all features without needing a real UK postcode. Review notes explaining how to trigger notifications. TestFlight beta testing phase before public submission |

### Phase 1 — Data Pipeline

| # | Feature | Details |
|---|---------|---------|
| 1.1 | PlanIt polling service | Background service polling `GET /api/applics/json?different_start={last_poll_iso}&pg_sz=5000&sort=-last_different` on configurable interval (default 15 min), with rate limit handling and exponential backoff on 429s. Handles pagination: if a response returns `pg_sz` results, follow `page` parameter to fetch subsequent pages until exhausted |
| 1.2 | Application ingestion | Diff polled results against Cosmos DB, idempotent upsert by PlanIt `name` field (`{area_name}/{uid}`, globally unique) into Applications container |
| 1.3 | Postcode geocoding | Integrate [postcodes.io](https://postcodes.io) to convert user-entered postcodes to lat/lng coordinates for watch zone storage. Required for spatial matching in Cosmos DB (ST_DISTANCE queries against application locations) |
| 1.4 | Watch zone matching | On ingestion, spatial match of application lat/lng against active user watch zones (stored as geocoded centre point + radius in Cosmos DB) |
| 1.5 | Polling scope management | Configure polling queries filtered by authority for areas with active users, expanding coverage as user base grows |
| 1.6 | Backfill (paid users only) | On zone creation for Personal/Pro users, query PlanIt `GET /api/applics/json` with location parameters to seed recent applications |
| 1.7 | Polling health monitoring | Automated alerts if polling fails or data freshness degrades beyond configurable thresholds. Track last successful poll timestamp, consecutive failures, and data staleness |
| 1.8 | Free-tier cold-start seed | On zone creation for free users, seed the map/list with a lightweight snapshot of recent applications already cached in Cosmos DB (no PlanIt API call — query existing data by spatial match). Prevents the empty-screen problem where free users see nothing until the next poll cycle covers their area. Distinct from paid backfill (1.6) which fetches fresh data from PlanIt |

### Phase 2 — Push Notifications (MVP)

| # | Feature | Details |
|---|---------|---------|
| 2.1 | APNs integration | Device token registration via API, push certificate management. Handle token lifecycle: process APNs feedback service responses to remove invalid/expired tokens, re-register on app launch to capture token rotation |
| 2.2 | Notification dispatch | Watch zone match → push notification via Cosmos DB change feed (see [ADR 0009](adr/0009-notification-delivery-architecture.md)). Enforce monthly cap (5/calendar month, resets 1st of each month) for free tier |
| 2.3 | Notification history | Stored in Cosmos DB, displayed as in-app feed |
| 2.4 | Notification preferences | Per-zone toggles: new applications (all tiers), status changes (Personal+), decision updates (Personal+) |

### Phase 3 — iOS App Experience

| # | Feature | Details |
|---|---------|---------|
| 3.1 | Onboarding flow | First-launch experience: welcome screens explaining the app's value, postcode entry, watch zone creation wizard (radius picker + map preview), notification permission prompt (deferred to this contextual moment, not on first launch). Guides users to a functional state before they hit the main UI |
| 3.2 | Map view | MapKit with application pins, colour-coded by status (Pending/Approved/Refused/Withdrawn) |
| 3.3 | Application detail | Address, description, status, dates, decision, link to council portal for public comments |
| 3.4 | Application list | Filterable list within watch zone. Free: browse only. Personal+: filter by status/type/date |
| 3.5 | Watch zone management | Add/edit/delete zones with postcode entry + radius picker + map preview. Free limited to 1 zone |
| 3.6 | Full-text search | Pro tier only. Pass-through to PlanIt search parameters |
| 3.7 | Deep links | Tap notification → opens relevant application detail screen |
| 3.8 | Offline caching | SwiftData local persistence of applications, watch zones, and notification history. App remains browsable without connectivity. Background sync on reconnect. Cache invalidation aligned with polling freshness (15–30 min TTL) |
| 3.9 | Settings & account screen | Centralised settings: account info (email, auth method), notification preferences, subscription management (links to iOS subscription settings), watch zone list, data attribution display (PlanIt, Crown Copyright, OS, OSM — see [Attribution Requirements](#attribution-requirements-mandatory)), privacy policy, Terms of Service, account deletion, app version |
| 3.10 | Empty & error states | Purposeful empty states for all list/map screens: map with no applications ("Checking for planning applications near you..."), notification feed with no history, search with no results. Error states for connectivity loss (beyond offline caching — explicit "No connection" banner with retry), API errors, and auth session expiry. Loading skeletons for data-fetching screens |
| 3.11 | Force-update mechanism | On app launch, check minimum supported API version against a server-side config value. If the installed app is below the minimum, show a blocking modal directing users to the App Store. Ties into API versioning (0.10) to safely deprecate old endpoints |

### Phase 4 — Engagement & Retention

| # | Feature | Details |
|---|---------|---------|
| 4.1 | Status timeline | Visual lifecycle: Pending → Approved / Refused / Withdrawn with dates |
| 4.2 | Decision alerts | Targeted notification when a bookmarked application gets a decision |
| 4.3 | Saved applications | Bookmark specific applications for ongoing monitoring (all tiers) |
| 4.4 | Weekly digest | Push summary: "X new applications near you this week" (Personal+ tier) |
| 4.5 | Authority directory | Browse/search 417 authorities from cached `GET /api/areas/json` data |

### Phase 5 — Growth

| # | Feature | Details |
|---|---------|---------|
| 5.1 | Dynamic polling optimisation | Adjust polling frequency and authority scope based on active user distribution to minimise PlanIt API load |
| 5.2 | Public comments (investigation) | Evaluate: deep links to council portals or selective scraping |
| 5.3 | Community groups | Shared watch zones for neighbourhood associations, parish councils |
| 5.4 | Gov.uk Planning Data adapter | Integrate Gov.uk Planning Data as supplementary source for boundary, designation, and conservation area data to enrich application context |

---

## Infrastructure

### Pulumi Stack Structure

Single Pulumi project (`/infra`) with one stack per environment. Start with a single **dev** stack; add **prod** when approaching first release.

| Resource | Purpose |
|----------|---------|
| Azure Resource Group | Logical container for all Town Crier resources |
| Azure Cosmos DB (Serverless) | Account + database + containers (Users, WatchZones, Applications, Notifications) with partition keys |
| Azure Container Apps Environment | Shared environment with consumption plan |
| Azure Container App (API) | Runs the .NET API container. Min replicas: 0 (dev), 1 (prod). Health probe on `/health` |
| Azure Container Registry (ACR) | Stores API Docker images. Basic SKU |
| Azure Log Analytics Workspace | Backing store for Container Apps logs and metrics |

### State & Secrets

- **Pulumi state:** Pulumi Cloud (free tier, single-user). No self-managed blob backend needed initially.
- **Application secrets:** Stored in GitHub Actions secrets, injected into Container App secrets at deploy time via Pulumi (`Secret` environment variables) or `az containerapp update --set-env-vars`. No Key Vault — unnecessary for a single service with a single deployment pipeline. Can be introduced later if multiple services need shared secrets or rotation policies.
- **Pulumi secrets provider:** Default Pulumi Cloud encryption (passphrase-free).

### Auth0

Auth0 resources (tenant, application, API, connections) are configured manually via the Auth0 Dashboard — not managed by Pulumi. The Auth0 domain and client ID are non-secret config values stored in Pulumi stack config. The client secret is stored as a GitHub Actions secret and injected at deploy time.

---

## CI/CD

### Pipeline Overview

All pipelines run in GitHub Actions. Three independent workflows, triggered by path filters:

| Workflow | Trigger paths | Runs on |
|----------|--------------|---------|
| `api.yml` | `api/**`, `.github/workflows/api.yml` | PR + push to `main` |
| `ios.yml` | `mobile/ios/**`, `.github/workflows/ios.yml` | PR + push to `main` |
| `infra.yml` | `infra/**`, `.github/workflows/infra.yml` | PR + push to `main` |

### API Pipeline (`api.yml`)

| Stage | Trigger | Steps |
|-------|---------|-------|
| Build & Test | PR, push to `main` | `dotnet format --verify-no-changes` → `dotnet build` → `dotnet test` |
| Publish image | Push to `main` only | `docker build` (Alpine, Native AOT) → tag with commit SHA → push to ACR |
| Deploy | Push to `main` only | Update Container App revision to new image tag via `az containerapp update` |

### iOS Pipeline (`ios.yml`)

| Stage | Trigger | Steps |
|-------|---------|-------|
| Build & Test | PR, push to `main` | `swiftlint lint --strict` → `swift build` → `swift test` (or `xcodebuild test`) |
| Archive & Distribute | Push to `main` only (later: tags) | `xcodebuild archive` → upload to TestFlight via `xcrun altool` or Xcode Cloud |

**Note:** iOS signing uses App Store Connect API key stored as a GitHub Actions secret. No Fastlane — keep dependencies minimal, use `xcodebuild` directly.

### Infrastructure Pipeline (`infra.yml`)

| Stage | Trigger | Steps |
|-------|---------|-------|
| Preview | PR | `pulumi preview --stack dev` — posts summary as PR comment |
| Deploy | Push to `main` | `pulumi up --stack dev --yes` |

### Environment Strategy

- **Dev** — single environment deployed on every push to `main`. Used for development and testing.
- **Prod** — added later (Phase 2/3 timeframe) as a separate Pulumi stack. Deployed via manual workflow dispatch or release tags.

No staging environment initially — unnecessary overhead for a solo developer. Prod deployment added when there are real users.

### Branch Protection

- `main` branch: require PR with passing checks before merge
- Required checks: the Build & Test stage of each affected workflow (path-filtered)
- No required reviewers initially (solo developer)

### Secrets Management

All application secrets live in **GitHub Actions secrets** and are injected into Container App secrets at deploy time. No Key Vault needed initially.

| Secret | GitHub Actions secret | Injected into |
|--------|----------------------|---------------|
| Auth0 client secret | `AUTH0_CLIENT_SECRET` | Container App env var |
| APNs signing key | `APNS_SIGNING_KEY` | Container App env var |
| Cosmos DB connection string | `COSMOS_CONNECTION_STRING` | Container App env var |
| ACR credentials | `ACR_USERNAME` / `ACR_PASSWORD` | API workflow (image push) |
| Pulumi access token | `PULUMI_ACCESS_TOKEN` | Infra workflow |
| App Store Connect API key | `APP_STORE_CONNECT_KEY` | iOS workflow (TestFlight upload) |

---

## Cross-Cutting Concerns

| Area | Approach |
|------|----------|
| API cost management | Cache all polled data in Cosmos DB. Serve browse/list/map/detail from own data. Only call PlanIt for backfill and full-text search |
| Polling reliability | Idempotent upserts keyed on PlanIt `name` field (`{area_name}/{uid}`). Track last successful poll timestamp for change detection |
| Polling scaling | Start with authority-scoped polls for areas with active users. Broaden coverage as user density grows |
| Rate limit handling | Respect 429 responses, exponential backoff. PlanIt rate limits are unpublished — poll conservatively |
| Data freshness | Polling provides 15–30 minute latency, acceptable for planning applications with week-long consultation periods |
| Tier enforcement | API enforces limits (notification cap, zone count, radius, search access). iOS app shows upgrade prompts at limit boundaries |
| PlanIt maintainer outreach | Contact Andrew Speakman (PlanIt maintainer) pre-launch to introduce the project, confirm acceptable use, and offer attribution/donation arrangement |
| API rate limiting | Per-user request throttling (see 0.13). Prevents abuse from free-tier clients and protects downstream resources |
| Data retention | TTL policy on Applications container for planning applications older than a configurable threshold (e.g., 2 years past decision date). Keeps Cosmos DB storage costs bounded as data accumulates. Archived data available via PlanIt if needed |
| Privacy / GDPR | Privacy policy and Terms of Service published in-app and on App Store listing. Data export and account deletion endpoints (see 0.12). Minimal data collection — no tracking beyond what's needed for core functionality. Cookie-free API (JWT bearer tokens only) |
| App Store compliance | Subscription disclosure (auto-renewal terms, price, cancellation), Restore Purchases button, account deletion, demo account for Apple reviewers, privacy nutrition labels. See 0.11, 0.12, 0.14 |
| Cold-start UX | Free users get spatial-matched cached data on zone creation (1.8). Paid users get PlanIt backfill (1.6). No user should see an empty screen after onboarding |
| Data attribution | PlanIt, Crown Copyright, OS, and OSM attributions displayed in Settings screen (3.9) and on map view. Required by data licensing terms |
