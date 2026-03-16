# 0003. External Data Provider Selection

Date: 2026-03-16

## Status
Superseded by [0006](0006-planit-primary-data-provider.md)

## Context
The "town-crier" application requires a comprehensive and reliable source of UK local authority planning application data. Manually scraping 379+ individual council portals is technically complex, resource-intensive, and difficult to maintain over time. We need a provider that offers high coverage, low latency for new applications, and a cost-effective entry point.

## Decision
We will use **PlanWire.io** as our primary external data provider for sourcing planning application data across the UK.

### Rationale
- **Comprehensive Coverage:** Supports 379 Local Planning Authorities (LPAs) across the UK, providing near-total national coverage from a single integration point.
- **Webhook Integration:** Offers native webhook support for `application.new` and `application.updated` events. This is critical for our push-notification-first architecture, as it allows for reactive data ingestion rather than expensive polling.
- **Cost-Effective Scaling:** 
    - A **Free Tier** (100 requests/day) allows for development and initial prototyping at £0 cost.
    - A **Starter Tier** (£29/mo for 1,000 requests/day) provides a clear and affordable path to production as the user base grows.
- **Rich Data & Search:** Provides normalized JSON data including addresses, geospatial coordinates (lat/lng), descriptions, and status updates, which simplifies our backend logic.
- **Developer-Friendly:** Standard REST API with HMAC-signed webhooks ensures secure and straightforward integration with our .NET 10 backend.

### Validation (2026-03-16)

Live API testing on the free tier confirmed PlanWire.io is a real, operational service returning genuine UK planning data.

### Endpoints Tested

| Endpoint | Result |
|----------|--------|
| `GET /v1/applications` | 200 — returned real data; 3,817,787 total applications |
| `GET /v1/applications/nearby` | 200 — spatial search (central London, 5km) returned relevant Camden results with `distanceM` |
| `GET /v1/applications/:id` | 200 — full detail including raw scraper output |
| `GET /v1/councils` | 200 — all councils with `lastScrapedAt` dates and application counts |

### Data Verification

Cross-referenced application `26/0019/S211` (Carlisle) against the live Cumberland Council planning portal. Reference, address, description, and status all matched exactly.

### Data Quality Observations

- **Freshness confirmed:** Council `lastScrapedAt` dates range from 1–5 days prior to testing. Active scraping is ongoing.
- **Underlying data source:** The `raw` field reveals PlanWire is built on top of PlanIt (planit.org.uk) scraper data, with PlanWire adding the API layer and webhook infrastructure. This is reassuring — PlanIt is a well-established ~20M application dataset. It also means a PlanIt fallback adapter would produce highly compatible data.
- **Null coordinates:** Some records (e.g. Aberdeenshire) have `null` lat/lng. The ingestion layer must handle missing geospatial data gracefully.
- **Inconsistent status strings:** One record contained raw HTML/whitespace in the status field (`"Status:\r\n  Awaiting decision"`). Status values need normalization on ingestion.
- **Sparse optional fields:** `applicationType`, `url`, and `documents` are frequently null/empty. The domain model should treat these as optional.
- **Response times:** 14–47 seconds observed on the free tier, far exceeding the documented <50ms median. Likely free-tier throttling or cold starts — needs re-evaluation on the Starter tier.

### Remaining Validation Gates

1. **Webhook delivery** — cannot test without a public endpoint and Starter tier (webhooks not available on Free). Validate once the API is deployed to Azure Container Apps.
2. **API performance under Starter tier** — confirm whether the slow response times are free-tier-specific.

## Consequences
- **Third-Party Dependency:** The application's core functionality depends on PlanWire's uptime and data accuracy.
- **Subscription Costs:** As the user base grows, the project will need to transition to paid tiers (£29/mo+), necessitating a monetization strategy (e.g., a low-cost subscription model) to remain sustainable.
- **Data Caching:** To minimize API request costs and stay within daily limits, the backend will implement a caching strategy in Cosmos DB for frequently accessed application data.
- **Webhook Security:** The .NET 10 API must implement HMAC-SHA256 signature verification to ensure the authenticity of incoming webhooks.
- **Data Normalization:** The ingestion layer must normalize inconsistent status strings, handle null coordinates, and treat optional fields (`applicationType`, `url`, `documents`) as nullable. This is standard ETL work, not a blocker.
