# Data Provider Comparative Assessment

**Date:** 2026-03-16
**Author:** Elfrida (Scribe, Data Provider Assessment Team)

---

## 1. Executive Summary

> **Correction (2026-03-16):** This assessment originally recommended PlanWire.io as the primary provider. Following hands-on validation (ADR 0003), PlanWire was found to be a paid wrapper around PlanIt's dataset. [ADR 0006](adr/0006-planit-primary-data-provider.md) switched the primary provider to **PlanIt** (free, polling-based ingestion). The analysis below remains useful as a provider comparison, but the recommendation is superseded.

This assessment originally recommended **PlanWire.io as the primary data provider**, with **Gov.uk Planning Data as a secondary/supplementary source** and **PlanIt as a tertiary fallback and validation reference**. This recommendation has been **superseded by ADR 0006**, which promotes PlanIt to the primary role.

PlanWire.io is a real, operational service with a live website, functional API documentation, signup flow, and code examples. It is the **only provider offering webhook support**, which is critical for Town Crier's push-notification-first architecture. The previous feasibility report's RED rating for the data provider was based on the conclusion that PlanWire.io "does not exist" — this assessment finds that conclusion to be **incorrect**. PlanWire.io exists, is operational, and its documented capabilities align closely with Town Crier's feature plan.

However, PlanWire.io has **no third-party reviews, community mentions, or independent verification** of its claims. This is a risk that must be mitigated through early integration testing and maintaining the adapter-based architecture that allows provider substitution.

---

## 2. Provider Profiles

### 2.1 PlanWire.io

**URL:** https://planwire.io | **Docs:** https://planwire.io/docs

PlanWire is a developer-focused REST API providing UK planning application data. It describes itself as built by developers frustrated with legacy vendors charging large sums for Excel exports. Data is sourced from daily bulk imports from the official DLUHC dataset plus direct council portal scraping.

**Key capabilities:**
- REST API with clean JSON responses, consistent pagination, meaningful error messages
- 379 LPAs indexed across England, Scotland, Wales, and Northern Ireland
- PostGIS-backed spatial search (radius queries up to 50km)
- Full-text search via PostgreSQL GIN indexes
- Native webhook support (`application.new`, `application.updated`) with HMAC-SHA256 signatures
- Webhook filtering by council, postcode prefix, status, application type
- Webhook delivery logs and test endpoint
- <50ms median response time (claimed)
- Daily data updates from official sources and council portal scraping

**Endpoints:**
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/v1/applications` | GET | List/filter applications |
| `/v1/applications/nearby` | GET | Spatial search (lat/lng + radius) |
| `/v1/applications/:id` | GET | Fetch by UUID |
| `/v1/applications/ref/:council/:reference` | GET | Fetch by council reference |
| `/v1/councils` | GET | List 379 LPAs with metadata |
| `/v1/councils/:id` | GET | Single council details |
| `/v1/webhooks` | GET/POST | List/create webhooks |
| `/v1/webhooks/:id` | DELETE | Remove webhook |
| `/v1/webhooks/:id/test` | POST | Test webhook delivery |
| `/v1/webhooks/:id/deliveries` | GET | View last 50 deliveries |

**Authentication:** API key via `X-API-Key` header or `Authorization: Bearer` header.

**Data source:** Crown Copyright data under OGL v3, sourced from planning.data.gov.uk plus council portal scraping.

**Risk factors:**
- No third-party reviews or independent mentions found via web search
- No /terms page (404 response)
- No published SLA, data retention policy, or liability terms
- Unknown company entity and registration details

### 2.2 PlanIt (planit.org.uk)

**URL:** https://www.planit.org.uk | **API Docs:** https://www.planit.org.uk/api

PlanIt (UK PlanIt) is a long-established national aggregator for UK planning applications, built and maintained by Andrew Speakman. It is a free service funded by donations, built on Django/PostgreSQL/PostGIS.

**Key capabilities:**
- ~20 million planning applications across 417 LPAs (England, Scotland, Wales, Northern Ireland)
- 91% of applications located on a map
- Rich spatial search: radius (km), bounding box, polygon boundary
- Full-text search in descriptions, application types, developer fields
- Comprehensive date filtering: submission, decision, changed, different dates
- Multiple output formats: JSON, GeoJSON, CSV, TSV, GeoRSS
- Up to 5,000 results per request
- Postcode-based search
- No authentication required

**Endpoints:**
| Endpoint | Purpose |
|----------|---------|
| `/api/applics/{fmt}` | Search applications (json, csv, geojson, tsv) |
| `/api/areas/{fmt}` | Search planning areas |
| `/planapplic/{id}/{fmt}` | Get single application |
| `/planarea/{area}/{fmt}` | Get area details |

**Limitations:**
- **No webhooks** — polling only
- **No API key / no authentication** — rate limiting enforced by IP
- Rate limits enforced but thresholds not published (429 responses)
- 5,000 result maximum per request, 1,000 KB content limit
- Free service with no SLA or uptime guarantees
- Run by a single developer — bus factor of 1
- No SDK or client libraries

### 2.3 Gov.uk Planning Data

**URL:** https://www.planning.data.gov.uk | **Docs:** https://www.planning.data.gov.uk/docs

The Planning Data platform is a government-run service by MHCLG (Ministry of Housing, Communities & Local Government) providing planning and housing data for England. It is currently in **beta** and explicitly not recommended for production systems.

**Key capabilities:**
- Government-backed, free, open data under OGL v3
- Entity-based data model covering multiple planning-related datasets
- Spatial search with DE-9IM relationships (within, intersects, contains, etc.)
- WKT geometry support for complex spatial queries
- GeoJSON output format
- Bulk data downloads (CSV, JSON, GeoJSON)
- OpenAPI specification available for client generation
- No authentication required

**Endpoints:**
| Endpoint | Purpose |
|----------|---------|
| `/entity.{ext}` | Search entities (json, geojson, html) |
| `/entity/{id}.{ext}` | Get single entity |
| `/dataset.{ext}` | List datasets |
| `/dataset/{name}.{ext}` | Get dataset details |
| `/entity/dataset-name-search.json` | Search entity names |

**Limitations:**
- **England only** — no Scotland, Wales, or Northern Ireland
- **Beta status** — explicitly experimental, may change without notice
- **No webhooks**
- **No planning application data as a first-class entity** — the platform focuses on planning policy, boundaries, and designations rather than individual planning applications
- Maximum 500 results per request
- Rate limits enforced but thresholds not published
- Limited filtering compared to PlanWire/PlanIt

---

## 3. Feature Comparison Matrix

| Feature | PlanWire.io | PlanIt (planit.org.uk) | Gov.uk Planning Data |
|---------|-------------|----------------------|---------------------|
| **Coverage (LPAs)** | 379 (all UK) | 417 (all UK) | England only |
| **Total applications** | Not published | ~20 million | N/A (entity-based) |
| **Application search** | Yes (filter by council, postcode, status, type, date, full-text) | Yes (filter by authority, postcode, radius, date, status, type, full-text) | Limited (entity search, not application-specific) |
| **Spatial search** | PostGIS radius (1-50km) | Radius (km), bbox, polygon | WKT geometry, DE-9IM relations |
| **Webhooks** | Yes (`application.new`, `application.updated`) | No | No |
| **Webhook filtering** | Council, postcode prefix, status, type | N/A | N/A |
| **Webhook security** | HMAC-SHA256 | N/A | N/A |
| **Full-text search** | Yes (GIN indexes) | Yes (quoted phrases, OR, negation) | Basic (postcode/UPRN) |
| **Data freshness** | Daily updates | Unknown (likely daily) | Varies by dataset |
| **Authentication** | API key | None (rate-limited by IP) | None |
| **Rate limits** | 100-unlimited/day by plan | Enforced, thresholds unpublished | Enforced, thresholds unpublished |
| **Pricing** | Free-£299/mo | Free (donations welcome) | Free |
| **Output formats** | JSON | JSON, GeoJSON, CSV, TSV, GeoRSS | JSON, GeoJSON, HTML |
| **Pagination** | Max 100/page | Max 5,000/page | Max 500/page |
| **SDK availability** | None (curl/JS/Python examples) | None (R package exists: `acton`) | OpenAPI spec for codegen |
| **Terms of use** | Not published (404) | OGL v3, copyright restrictions on plans/drawings | OGL v3 |
| **SLA** | Enterprise tier mentions SLA | None | None (beta) |
| **Uptime guarantee** | No published figure | No | No |
| **Operator** | Unknown company | Solo developer (Andrew Speakman) | UK Government (MHCLG) |
| **Longevity risk** | Unknown (new service) | Established (years of operation) | Government-backed (low risk) |

---

## 4. Alignment with Town Crier Requirements

### 4.1 Watch Zones (spatial monitoring)

| Requirement | PlanWire | PlanIt | Gov.uk |
|-------------|----------|--------|--------|
| Radius-based spatial search | Excellent (`/nearby` endpoint, 1-50km) | Excellent (`krad` parameter) | Partial (WKT geometry) |
| Postcode-to-location | Via postcode parameter | Via `pcode` parameter | Via `q` parameter |
| Multiple zone support | Yes (multiple webhook subscriptions) | Yes (multiple poll queries) | Limited |

**Selected: PlanIt** — excellent spatial search via `krad` parameter. Multiple zones supported via multiple poll queries. PlanWire's webhook filtering was appealing but not worth the cost given PlanIt is the upstream source.

### 4.2 Push Notifications (real-time alerting)

| Requirement | PlanWire | PlanIt | Gov.uk |
|-------------|----------|--------|--------|
| New application events | Webhook: `application.new` | Poll only | Poll only |
| Status change events | Webhook: `application.updated` | Poll with `changed` filter | No |
| Latency | Near-real-time (webhook) | 15-30 min (poll interval) | Hours+ (batch updates) |

**Selected: PlanIt** — requires a polling service (15–30 min interval), but planning consultation periods are measured in weeks so the latency is acceptable. The polling service is a one-time development cost that eliminates the £29–99/mo PlanWire subscription.

### 4.3 Backfill (seeding historical data for new zones)

| Requirement | PlanWire | PlanIt | Gov.uk |
|-------------|----------|--------|--------|
| Historical spatial query | Yes (`/nearby` with date filters) | Yes (`krad` + `start_date`) | Limited |
| Bulk retrieval | 100/page | Up to 5,000/page | 500/page |

**Selected: PlanIt** — 5,000 results/page makes backfill fast and efficient. One-time spatial query per zone creation for paid users.

### 4.4 Search (full-text application search)

| Requirement | PlanWire | PlanIt | Gov.uk |
|-------------|----------|--------|--------|
| Full-text search | Yes (`q` parameter, GIN indexes) | Yes (`search` parameter, quoted phrases, OR, negation) | Basic only |
| Filter by status/type | Yes | Yes | Limited |

**Selected: PlanIt** — rich full-text search with quoted phrases, OR, and negation operators. Pro tier passes search queries through to PlanIt.

### 4.5 Tier Enforcement (API cost management)

| Requirement | PlanWire | PlanIt | Gov.uk |
|-------------|----------|--------|--------|
| Free tier cost for browse/list | £0 (webhook data cached) | £0 (poll data cached) | £0 |
| Backfill cost (per zone) | 1 API call (Starter+) | 1 API call (free) | N/A |
| Search passthrough cost | Counts against daily limit | Rate-limited by IP | Free |

**Selected: PlanIt** — polling data is cached in Cosmos DB regardless of tier. Free-tier users read from the shared cache at zero marginal cost. No per-request charges from PlanIt. This aligns with the feature plan's zero-cost free tier strategy.

---

## 5. Recommended Data Strategy

> **Updated (2026-03-16):** This section originally recommended PlanWire as primary. [ADR 0006](adr/0006-planit-primary-data-provider.md) switched to PlanIt after discovering PlanWire is a paid wrapper over PlanIt's dataset.

### Primary: PlanIt (planit.org.uk)

**Rationale:** PlanIt is the upstream data source — PlanWire's `raw` field contains PlanIt scraper data, making PlanWire a paid intermediary over free data. PlanIt has wider coverage (417 vs 379 LPAs), larger page sizes (5,000 vs 100/page), no API key management, and zero cost. The trade-off is no webhook support, requiring a polling service — but planning consultation periods are measured in weeks, so 15–30 minute polling latency is acceptable.

**Confidence level:** High. PlanIt is established (years of operation), has downstream consumers (PlanWire itself, the `acton` R package), and was validated via live API calls (see ADR 0006).

### Secondary: Gov.uk Planning Data

**Rationale:** Government-backed, free, and improving over time. While it doesn't currently serve individual planning applications well, it provides valuable supplementary data (planning boundaries, designations, conservation areas) that could enrich the Town Crier experience. As the platform matures beyond beta, it may become a primary source.

**Use cases:** Supplementary boundary/designation data, validation of LPA coverage, fallback reference data.

---

## 6. Impact on Current Architecture

> **Updated (2026-03-16):** PlanIt is now the primary provider per [ADR 0006](adr/0006-planit-primary-data-provider.md). ADR 0003 (PlanWire as primary) is superseded.

### Data Provider Rating: GREEN

PlanIt was validated via live API calls (see ADR 0006 for full results). All critical endpoints are operational, response times are acceptable, and data quality is confirmed against known planning applications.

### Architecture Changes (from original PlanWire design)

- **Polling service replaces webhook receiver** — a background service in Azure Container Apps polls PlanIt on a configurable interval (default: 15 minutes) using `different_start` date filters for change detection.
- **ADR 0003 superseded by ADR 0006** — the webhook-driven ingestion model is replaced by polling-based ingestion.
- **Phase 1 of the feature plan updated** — polling service, change detection, and idempotent upserts instead of webhook receiver and HMAC verification.

### Architecture Additions

1. **`IPlanningDataProvider` port interface** — abstract the data provider behind a clean port for provider substitution.
2. **Gov.uk Planning Data adapter** — for supplementary boundary/designation data.
3. **Polling health monitoring** — automated alerts if polling fails or data freshness degrades.

---

## 7. Cost Comparison

> **Updated (2026-03-16):** Cost projections updated to reflect PlanIt as primary provider (£0 data provider cost). See [feature plan](feature-plan.md) for the current baseline.

### Monthly Infrastructure Costs (PlanIt as Primary)

| Cost Component | 0-100 users | 100-1,000 users | 1,000-10,000 users |
|---------------|-------------|-----------------|-------------------|
| **PlanIt data provider** | £0 | £0 | £0 |
| **Cosmos DB Serverless** | £5-10/mo | £10-25/mo | £25-60/mo |
| **Azure Container Apps** | £5-10/mo | £10-20/mo | £20-50/mo |
| **Auth0** | £0 | £0 | £0 (under 25K MAU) |
| **Apple Developer Program** | £6.60/mo | £6.60/mo | £6.60/mo |
| **Total** | **£17-27/mo** | **£27-52/mo** | **£52-117/mo** |

Note: Container Apps costs are slightly higher than the original PlanWire estimate because the polling service runs continuously, whereas webhook ingestion was event-driven. However, this is more than offset by eliminating the £29-99/mo PlanWire subscription.

### Revenue vs Cost at Each Scale

| Scale | Est. Paying Users (10% conversion) | Est. Monthly Revenue (net of Apple 15%) | Monthly Cost | Net |
|-------|-------------------------------------|----------------------------------------|-------------|-----|
| 100 users | 10 (8 Personal + 2 Pro) | £23.70 | ~£22/mo | +£1.70 |
| 1,000 users | 100 (80 Personal + 20 Pro) | £237 | ~£40/mo | +£197 |
| 10,000 users | 1,000 (800 Personal + 200 Pro) | £2,370 | ~£85/mo | +£2,285 |

**Key insight:** With PlanIt at £0, costs scale purely with infrastructure (Cosmos DB + Container Apps). Free-tier users generate zero marginal cost — they read from the shared polling cache. Paid-tier users only trigger additional PlanIt calls for backfill (one-time per zone) and full-text search (occasional, Pro tier only). The economics are significantly better than the original PlanWire projections at every scale above 100 users.

---

## Sources

- PlanWire.io documentation: https://planwire.io/docs
- PlanWire.io homepage: https://planwire.io
- PlanIt API documentation: https://www.planit.org.uk/api
- PlanIt acknowledgements: https://www.planit.org.uk/acknowledgements
- Gov.uk Planning Data documentation: https://www.planning.data.gov.uk/docs
- Gov.uk Planning Data OpenAPI spec: https://www.planning.data.gov.uk/openapi.json
- PlanIt R package (third-party): https://cyipt.github.io/acton/reference/get_planit_data.html
- UKPlanning scraper (PlanIt source): https://github.com/aspeakman/UKPlanning

---

*Assessment compiled 2026-03-16 by the Town Crier Data Provider Assessment Team. All findings based on direct inspection of live websites and documentation.*
