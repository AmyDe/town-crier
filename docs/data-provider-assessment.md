# Data Provider Comparative Assessment

**Date:** 2026-03-16
**Author:** Elfrida (Scribe, Data Provider Assessment Team)

---

## 1. Executive Summary

Town Crier should use **PlanWire.io as the primary data provider**, with **Gov.uk Planning Data as a secondary/supplementary source** and **PlanIt as a tertiary fallback and validation reference**.

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

**Best fit: PlanWire** — native webhook filtering by postcode prefix means new applications in a watch zone trigger automatic notifications without polling.

### 4.2 Push Notifications (real-time alerting)

| Requirement | PlanWire | PlanIt | Gov.uk |
|-------------|----------|--------|--------|
| New application events | Webhook: `application.new` | Poll only | Poll only |
| Status change events | Webhook: `application.updated` | Poll with `changed` filter | No |
| Latency | Near-real-time (webhook) | 15-30 min (poll interval) | Hours+ (batch updates) |

**Best fit: PlanWire** — webhooks are a fundamental differentiator. Without them, Town Crier must implement a polling service, adding complexity and latency.

### 4.3 Backfill (seeding historical data for new zones)

| Requirement | PlanWire | PlanIt | Gov.uk |
|-------------|----------|--------|--------|
| Historical spatial query | Yes (`/nearby` with date filters) | Yes (`krad` + `start_date`) | Limited |
| Bulk retrieval | 100/page | Up to 5,000/page | 500/page |

**Best fit: PlanIt** for bulk backfill (5,000/page vs 100/page), but PlanWire is adequate for the smaller per-zone backfills Town Crier needs.

### 4.4 Search (full-text application search)

| Requirement | PlanWire | PlanIt | Gov.uk |
|-------------|----------|--------|--------|
| Full-text search | Yes (`q` parameter, GIN indexes) | Yes (`search` parameter, quoted phrases, OR, negation) | Basic only |
| Filter by status/type | Yes | Yes | Limited |

**Best fit: Tie** — both PlanWire and PlanIt offer rich full-text search. PlanIt's search syntax is slightly more advanced (negation, OR operators).

### 4.5 Tier Enforcement (API cost management)

| Requirement | PlanWire | PlanIt | Gov.uk |
|-------------|----------|--------|--------|
| Free tier cost for browse/list | £0 (webhook data cached) | £0 (poll data cached) | £0 |
| Backfill cost (per zone) | 1 API call (Starter+) | 1 API call (free) | N/A |
| Search passthrough cost | Counts against daily limit | Rate-limited by IP | Free |

**Best fit: PlanWire** — webhook-driven caching means free-tier users consume zero PlanWire API calls for browse/list. This aligns perfectly with the feature plan's zero-cost free tier strategy.

---

## 5. Recommended Data Strategy

### Primary: PlanWire.io

**Rationale:** PlanWire is the only provider offering webhooks, which are architecturally critical for Town Crier's push-notification-first design. The webhook-driven model eliminates polling complexity, reduces latency, and enables the zero-cost free tier strategy described in the feature plan. The pricing (£29/mo Starter) is already budgeted.

**Confidence level:** Medium-high. The service exists and its documentation is comprehensive, but the lack of third-party verification is a concern. Mitigate by: (1) signing up for the free tier immediately and testing all endpoints, (2) verifying webhook delivery reliability over a 2-week trial, (3) maintaining the adapter architecture for easy provider substitution.

### Secondary: Gov.uk Planning Data

**Rationale:** Government-backed, free, and improving over time. While it doesn't currently serve individual planning applications well, it provides valuable supplementary data (planning boundaries, designations, conservation areas) that could enrich the Town Crier experience. As the platform matures beyond beta, it may become a primary source.

**Use cases:** Supplementary boundary/designation data, validation of LPA coverage, fallback reference data.

### Tertiary: PlanIt (planit.org.uk)

**Rationale:** PlanIt has the largest historical dataset (~20M applications) and the widest LPA coverage (417 vs 379). It serves as an excellent validation source and fallback if PlanWire experiences issues. Its generous page sizes (5,000/request) make it ideal for one-time bulk operations.

**Use cases:** Data validation/cross-referencing, bulk backfill if PlanWire is slow, fallback provider if PlanWire becomes unavailable, coverage gap filling (38 additional LPAs).

---

## 6. Impact on Current Architecture

### PlanWire Confirmed: Feasibility Report RED Rating Should Be Revised to GREEN

The feasibility report (2026-03-16) rated the data provider as RED based on the finding that "PlanWire.io is not a real service." This assessment finds that:

1. **PlanWire.io has a live website** with functional navigation, pricing page, and documentation
2. **The API documentation is detailed and internally consistent**, including endpoints, error codes, pagination, webhook configuration, and code examples in multiple languages
3. **The homepage describes real data sourcing** — daily bulk imports from DLUHC plus council portal scraping
4. **The service offers a signup flow** with API key generation

**Revised rating: AMBER** (not yet GREEN). While PlanWire appears real, the lack of independent third-party verification means the rating should be upgraded to AMBER pending hands-on validation. It should move to GREEN once the team has:
- Successfully created a free-tier account and received an API key
- Made live API calls and verified response data against known planning applications
- Registered a test webhook and confirmed delivery
- Verified data freshness (daily updates as claimed)

### Architecture Changes Required

**If PlanWire is validated (expected):**
- **Minimal changes needed.** The current architecture in the feature plan (ADR 0003, Phase 1) was designed around PlanWire and can proceed as-is.
- The hexagonal architecture's port/adapter pattern should still be implemented to allow provider substitution, but the webhook-driven ingestion model remains viable.
- ADR 0003 status remains "Accepted."

**If PlanWire validation fails:**
- Fall back to the feasibility report's recommended strategy: PlanIt as primary with polling-based ingestion.
- ADR 0003 would need to be superseded.
- Phase 1 of the feature plan would need significant revision (polling service instead of webhook receiver).

### Recommended Architecture Additions (regardless of PlanWire outcome)

1. **`IPlanningDataProvider` port interface** — abstract the data provider behind a clean port, as the feasibility report recommends. This is good practice even with a validated primary provider.
2. **Gov.uk Planning Data adapter** — for supplementary boundary/designation data.
3. **PlanIt adapter** — for fallback/validation scenarios.
4. **Health monitoring** — implement PlanWire webhook delivery monitoring and automated alerts if delivery stops.

---

## 7. Cost Comparison

### Monthly Infrastructure + Data Provider Costs

| Cost Component | 0-100 users | 100-1,000 users | 1,000-10,000 users |
|---------------|-------------|-----------------|-------------------|
| **PlanWire plan** | Free (£0) | Starter (£29/mo) | Growth (£99/mo) |
| **PlanWire rationale** | 100 req/day sufficient for testing + limited backfill | 1,000 req/day + 5 webhooks covers backfill + search | 10,000 req/day + unlimited webhooks for scale |
| **PlanIt** | £0 | £0 | £0 |
| **Gov.uk Planning Data** | £0 | £0 | £0 |
| **Cosmos DB Serverless** | £5-10/mo | £10-25/mo | £25-60/mo |
| **Azure Container Apps** | £0-5/mo | £5-15/mo | £15-40/mo |
| **Auth0** | £0 | £0 | £0 (under 25K MAU) |
| **Apple Developer Program** | £6.60/mo | £6.60/mo | £6.60/mo |
| **Total baseline** | **£12-22/mo** | **£51-76/mo** | **£147-206/mo** |

### Revenue vs Cost at Each Scale

| Scale | Est. Paying Users (10% conversion) | Est. Monthly Revenue (net of Apple 15%) | Monthly Cost | Net |
|-------|-------------------------------------|----------------------------------------|-------------|-----|
| 100 users | 10 (8 Personal + 2 Pro) | £23.70 | ~£17/mo | +£6.70 |
| 1,000 users | 100 (80 Personal + 20 Pro) | £237 | ~£63/mo | +£174 |
| 10,000 users | 1,000 (800 Personal + 200 Pro) | £2,370 | ~£176/mo | +£2,194 |

**Key insight:** The webhook-driven architecture with PlanWire keeps marginal per-user costs extremely low. Free-tier users generate zero data provider API calls (served from cached webhook data). Paid-tier users only trigger API calls for backfill (one-time) and search (occasional). This means costs scale primarily with infrastructure (Cosmos DB + Container Apps), not with the data provider.

### Comparison: PlanWire Strategy vs Polling-Only Strategy

| Aspect | PlanWire (webhooks) | PlanIt Polling |
|--------|-------------------|---------------|
| Data provider cost | £0-299/mo (by plan) | £0 |
| Polling compute cost | £0 (no polling needed) | £10-50/mo (Container Apps Jobs) |
| Development complexity | Lower (webhook receiver) | Higher (polling scheduler, change detection, deduplication) |
| Notification latency | Near-real-time | 15-30 minutes |
| Free tier viability | Zero marginal cost | Low marginal cost (polling runs regardless) |
| Total cost (1K users) | ~£63/mo | ~£35-55/mo |

The polling-only strategy is ~£10-30/mo cheaper at the 1,000-user scale, but adds significant development complexity and sacrifices the near-real-time notification experience that differentiates Town Crier from competitors.

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
