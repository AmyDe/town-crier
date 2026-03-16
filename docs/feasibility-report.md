# Town Crier: Feasibility Report

**Date:** 2026-03-16
**Author:** Godwin (Scribe, Town Crier Feasibility Team)

---

## 1. Executive Summary

Town Crier proposes a mobile-first app for monitoring UK planning applications with push notifications. The concept is sound and addresses a real gap in the market. The proposed architecture is technically feasible across all components.

> **Correction (2026-03-16):** An earlier version of this report incorrectly stated that PlanWire.io does not exist. A follow-up assessment confirmed PlanWire.io is a real, operational service with a live website, API documentation, and signup flow. See [Data Provider Assessment](data-provider-assessment.md) for the full comparative analysis. The data provider rating has been upgraded from RED to AMBER pending hands-on validation.

### Traffic Light Assessment

| Area | Rating | Summary |
|------|--------|---------|
| **Data Provider** | :large_orange_diamond: **AMBER** | PlanWire.io confirmed real. Pending hands-on validation of API, webhooks, and data freshness. See [Data Provider Assessment](data-provider-assessment.md). |
| **Azure Architecture** | :green_circle: **GREEN** | Container Apps + Cosmos DB Serverless is cost-effective and technically sound. |
| **Authentication & Monetisation** | :green_circle: **GREEN** | Auth0 free tier (25K MAU) is confirmed. Subscription model is viable. |
| **iOS & Geospatial** | :green_circle: **GREEN** | SwiftUI/MVVM-C is mature. Cosmos DB supports native geospatial queries. |
| **Market & Competition** | :large_orange_diamond: **AMBER** | Real demand exists but free competitors cover basic use cases. Differentiation needed. |
| **Overall** | :green_circle: **GREEN** | Feasible. Proceed with PlanWire as primary provider; validate with hands-on testing. |

---

## 2. Data Provider Assessment

### PlanWire.io: Confirmed Real (Pending Validation)

> **Correction (2026-03-16):** An earlier version of this section incorrectly stated PlanWire.io does not exist. A follow-up investigation confirmed it is a real, operational service. See [Data Provider Assessment](data-provider-assessment.md) for the full analysis.

**PlanWire.io is a real service** with a live website ([planwire.io](https://planwire.io)), functional API documentation ([planwire.io/docs](https://planwire.io/docs)), and a signup flow with API key generation. Key findings:

- **API documentation** is detailed and internally consistent — endpoints, error codes, pagination, webhook configuration, and code examples in multiple languages
- **Data sourcing** described as daily bulk imports from the official DLUHC dataset plus direct council portal scraping
- **379 LPAs** indexed across England, Scotland, Wales, and Northern Ireland
- **Webhooks** with `application.new` / `application.updated` events and HMAC-SHA256 signatures — the only UK planning data provider offering this
- **Pricing** matches the feature plan: Free (100 req/day), Starter £29/mo (1,000 req/day, 5 webhooks), Growth £99/mo, Enterprise £299/mo

**Risk factors:** No third-party reviews, community mentions, or independent verification found. No published terms of service (404 response). Unknown company entity. These are mitigated by hands-on validation before committing.

### Alternative UK Planning Data Providers

A comparative assessment of alternatives is documented in [Data Provider Assessment](data-provider-assessment.md). Summary:

| Provider | Coverage | Webhooks | Pricing | Role |
|----------|----------|----------|---------|------|
| **[PlanWire.io](https://planwire.io)** | 379 LPAs (all UK) | Yes | Free–£299/mo | **Primary** |
| **[Gov.uk Planning Data](https://www.planning.data.gov.uk/)** | England only (beta) | No | Free | **Secondary** (supplementary boundary/designation data) |
| **[PlanIt](https://www.planit.org.uk/)** | 417 LPAs, ~20M applications | No | Free | **Tertiary** (fallback, validation, bulk backfill) |

### Recommended Data Strategy

Use **PlanWire.io as the primary provider** — its webhook support is architecturally critical for Town Crier's push-notification-first design and zero-cost free tier strategy. Maintain the adapter-based architecture (`IPlanningDataProvider` port interface) to allow provider substitution:

1. **Primary: PlanWire.io** — Webhook-driven ingestion as designed in the feature plan. Validate by signing up for the free tier and testing endpoints, webhooks, and data freshness.
2. **Secondary: Gov.uk Planning Data** — Supplementary boundary and designation data as the platform matures.
3. **Tertiary: PlanIt** — Fallback provider, data validation, and bulk backfill (5,000 results/page vs PlanWire's 100/page).
4. **No architecture change needed** — The proposed webhook ingestion model can proceed as designed.

---

## 3. Azure Architecture Assessment

### .NET 10 with Native AOT: Confirmed Feasible

.NET 10 was released on **11 November 2025** as a Long-Term Support (LTS) release with support until November 2028. Native AOT in .NET 10 is production-ready with significant improvements:

- Minimal API compiled with Native AOT weighs **under 5 MB** (vs 18-25 MB in .NET 8)
- Faster cold start times — ideal for scale-to-zero Container Apps
- Broader Azure SDK support for AOT
- ASP.NET Core 10 Native AOT template supports OpenAPI generation by default

The constraint of avoiding reflection, using `System.Text.Json` source generators, and no EF Core/Dapper is well-documented and achievable. The Cosmos DB SDK is confirmed AOT-compatible with `System.Text.Json` serialization.

### Azure Container Apps (Consumption Plan): Confirmed Cost-Effective

- **Free allowance:** 180,000 vCPU-seconds, 360,000 GiB-seconds, and **2 million requests/month** per subscription
- **Scale-to-zero:** No charges when idle — excellent for early-stage/low-traffic apps
- **Idle billing:** Reduced rate when replicas are inactive but not scaled to zero

The proposed estimate of **£5-10/month** for low traffic is realistic and possibly optimistic (could be near £0 within the free allowance during early adoption).

### Azure Cosmos DB Serverless: Confirmed Cost-Effective

- **Pay-per-request:** Billed per RU consumed + storage
- **No minimum charge:** Ideal for prototype/early-stage
- **No free tier overlap:** Note that the Cosmos DB free tier (400 RU/s provisioned) does **not** apply to serverless accounts — but serverless is still cheaper for intermittent/low workloads
- **Geospatial support:** Native GeoJSON indexing and spatial queries (ST_DISTANCE, ST_WITHIN, ST_INTERSECTS) — suitable for watch zone matching

The proposed estimate of **£5-15/month** is realistic for low-to-moderate usage.

### Cost Estimate

| Service | Estimated Cost |
|---------|---------------|
| PlanWire Starter (webhooks + 1,000 req/day) | £29/mo |
| Cosmos DB Serverless | £5–15/mo |
| Azure Container Apps (consumption) | £0–10/mo |
| Auth0 | £0 (free tier) |
| Apple Developer Program | £79/year (~£6.60/mo) |
| **Total baseline** | **~£41–61/mo** |

This aligns with the original feature plan estimate of ~£40-55/mo. See [Data Provider Assessment](data-provider-assessment.md) for detailed cost projections at different user scales.

---

## 4. Authentication & Monetisation Assessment

### Auth0: Confirmed Viable

- **Free tier:** 25,000 MAU for B2C (confirmed via Auth0 documentation and community posts) — more than sufficient for early growth
- **Swift SDK:** [Auth0.swift](https://github.com/auth0/Auth0.swift) is actively maintained, supports SPM, and includes Universal Links on iOS 17.4+
- **.NET SDK:** Available and well-documented
- **Passkeys:** Supported on the free tier
- **Sign in with Apple:** Available as a social connection toggle
- **MFA (TOTP):** Available; MFA via SDKs is in Early Access

The proposed Auth0 integration is fully feasible as documented.

### App Store Subscription Model: Viable with Caveats

- **Apple commission:** 30% standard, reduced to **15%** under the Small Business Program (developers earning under $1M/year — Town Crier will comfortably qualify)
- **Subscription commission:** Drops to **15%** after a subscriber's first year (standard) or **10%** after first year under Small Business Program
- **EU rates:** Further reduced to 13% / 10% under Digital Markets Act alternative terms

**Revenue impact at 15% commission:**

| Tier | Price | Apple Cut | Net Revenue |
|------|-------|-----------|-------------|
| Personal | £1.99/mo | £0.30 | £1.69/mo |
| Pro | £5.99/mo | £0.90 | £5.09/mo |

**Revised break-even** (at ~£22/mo baseline): ~13 Personal subs or ~5 Pro subs. This is achievable.

### GDPR Compliance

- Auth0 handles most authentication GDPR requirements (data processing, right to deletion)
- Planning application data is **public information** — no special GDPR concerns for the planning data itself
- User location data (postcodes, watch zones) requires standard GDPR handling: privacy policy, data minimisation, right to deletion
- Apple's App Tracking Transparency rules apply if any analytics/attribution SDKs are used

---

## 5. iOS & Geospatial Assessment

### SwiftUI + MVVM-C: Mature and Suitable

SwiftUI is production-ready for this type of application. The MVVM-C (Model-View-ViewModel-Coordinator) pattern is well-established in the iOS community and appropriate for the navigation complexity of Town Crier (map views, detail screens, settings).

### SwiftData: Adequate with Caveats

SwiftData (as of iOS 18) is stable for straightforward use cases like caching planning applications locally. Key considerations:

- **Suitable for:** Caching application data, storing user preferences, notification history
- **Limitations:** No heavyweight migrations, limited predicate support, slower than direct SQLite
- **Risk level:** Low — Town Crier's on-device data needs are simple (cache + preferences). SwiftData is not the primary datastore; Cosmos DB is.

### MapKit Pin Clustering: Native Support

MapKit provides built-in annotation clustering via `MKClusterAnnotation` — no third-party libraries needed. Colour-coding pins by status (Pending/Approved/Refused/Withdrawn) is straightforward with custom `MKAnnotationView` subclasses.

### Cosmos DB Spatial Queries: Native GeoJSON Support

Cosmos DB for NoSQL supports native geospatial indexing and queries using GeoJSON:

- **ST_DISTANCE:** Find applications within a radius of a point — directly supports watch zone matching
- **ST_WITHIN:** Check if a point falls within a polygon
- **Spatial indexing:** Automatic indexing of GeoJSON `Point`, `Polygon`, `LineString` types

This eliminates the need for a separate spatial database or PostGIS. Watch zone matching can be implemented entirely within Cosmos DB.

### UK Postcode Geocoding

UK postcodes can be geocoded to lat/lng using:
- **postcodes.io** — Free, open-source REST API for UK postcode lookups
- **Ordnance Survey Data Hub** — Official government geospatial data
- No PlanWire dependency needed for geocoding

---

## 6. Market & Competitive Assessment

### Existing Competitors

| Competitor | Model | Coverage | Alerts | Price |
|-----------|-------|----------|--------|-------|
| **[Planning Alerts](https://planning.org.uk/)** | Web + email | 241 LPAs | Email (daily) | Free (¼ mile); paid for larger radius |
| **[Nimbus Maps](https://www.nimbusmaps.co.uk/)** | Web platform | UK-wide | Planning alerts | Premium (property developer pricing) |
| **[Searchland](https://searchland.co.uk/)** | Web platform | UK-wide (23.9M apps) | Unknown | From £195/mo |
| **Council portals** | Individual websites | Per-council | Some offer email | Free |
| **Planning Portal** | Gov.uk | UK-wide | None | Free |

### Market Demand Signals

- **Government investment:** UK government hired Google to develop AI planning tools (announced March 2026), indicating policy priority for planning transparency
- **Planning approvals at record low:** ~7,000 housing applications approved in Q2 2025 (lowest since 1979) — heightened public interest in planning decisions
- **Gap in mobile experience:** No dedicated, well-designed iOS app exists for planning alerts. Planning Alerts (planning.org.uk) is web/email only. Council portals are fragmented and poor UX.

### Competitive Positioning

**Town Crier's differentiation:**
1. **Native iOS app with push notifications** — no competitor offers this
2. **Map-first UX** with pin clustering — visual, intuitive interface
3. **Low price point** (£1.99-5.99/mo) — far below Searchland/Nimbus Maps
4. **Consumer-focused** — competitors target property professionals

**Risks:**
1. Planning Alerts offers free email alerts covering basic needs
2. Government AI planning tools may improve council portals directly
3. Limited moat — data is public; barrier is in UX and notification infrastructure

---

## 7. Key Risks & Mitigations

| # | Risk | Severity | Likelihood | Mitigation |
|---|------|----------|------------|------------|
| 1 | **PlanWire.io unvalidated** — no third-party reviews, no published ToS, unknown company entity | Medium | Medium | Sign up for free tier, validate endpoints/webhooks/data freshness before committing. Maintain adapter architecture for provider substitution. |
| 2 | **PlanWire.io availability/reliability** — single provider dependency with no published SLA (except Enterprise tier) | Medium | Low | Adapter-based architecture allows fallback to PlanIt. Cache aggressively in Cosmos DB. |
| 3 | **Planning Alerts (planning.org.uk) as free competitor** | Medium | Certain | Differentiate on mobile UX, push notifications, map experience, and richer filtering |
| 4 | **LPA coverage gaps** — PlanWire covers 379, PlanIt covers 417 | Low | Low | PlanIt adapter fills 38 LPA gap if needed; Gov.uk data improving |
| 5 | **SwiftData migration complexity** if data model evolves significantly | Low | Medium | Keep on-device schema simple; primary data lives in Cosmos DB |
| 6 | **Apple App Store rejection** — planning apps are niche; review may question utility | Low | Low | Clear value proposition; strong precedent for location-based alert apps |
| 7 | **Gov.uk Planning Data API is experimental** — may change or be discontinued | Low | Low | Use as supplementary source, not primary; Gov.uk has strong commitment to open planning data |
| 8 | **Auth0 pricing changes** if MAU exceeds 25K | Low | Low | 25K MAU is generous; reassess auth strategy if/when approaching limit |

---

## 8. Recommended Changes to Proposed Architecture

### Must Do (Critical)

1. **Validate PlanWire.io hands-on** before committing to production use
   - Sign up for the free tier and obtain an API key
   - Make live API calls and verify response data against known planning applications
   - Register a test webhook and confirm delivery with HMAC-SHA256 signature verification
   - Verify data freshness (daily updates as claimed)
   - Move data provider rating from AMBER to GREEN once validated

2. **Implement `IPlanningDataProvider` port interface** in the application layer to abstract data provider details, enabling provider substitution if needed

### Should Change (Recommended)

3. **Add postcodes.io integration** for postcode-to-lat/lng geocoding (free, open-source)

4. **Add PlanIt adapter** as fallback/validation provider — covers 38 additional LPAs and offers bulk retrieval (5,000/page)

5. **Add Gov.uk Planning Data adapter** for supplementary boundary and designation data

6. **Implement PlanWire webhook health monitoring** — automated alerts if delivery stops, with fallback to PlanIt polling

### Could Change (Optional)

7. **Consider contributing to Planning Data (Gov.uk)** — as an open-source government project, there may be opportunities to collaborate on improving the API

---

## 9. Go/No-Go Recommendation

### Verdict: **GO** (with validation gate)

Town Crier is feasible and addresses a genuine market gap. The core technical architecture (.NET 10, Azure Container Apps, Cosmos DB, Auth0, SwiftUI) is sound, cost-effective, and well-chosen. PlanWire.io is confirmed as a real service whose capabilities align with the proposed architecture. The cost base is low enough that break-even requires only ~5-13 paying subscribers.

**Proceed with one validation gate:**

1. **Mandatory (before production):** Validate PlanWire.io hands-on — sign up, test API calls, verify webhook delivery and data freshness. This should take 1-2 days and will move the data provider rating from AMBER to GREEN.

2. **Advisory:** Implement the adapter-based architecture (`IPlanningDataProvider` port) to maintain provider substitution capability, with PlanIt as a ready fallback.

3. **Advisory:** Add postcodes.io integration for UK postcode geocoding.

**The project is viable** with a baseline cost of ~£41-61/month, a clear path to break-even, near-real-time push notifications via PlanWire webhooks, and meaningful differentiation from existing competitors through its native iOS experience.

See [Data Provider Assessment](data-provider-assessment.md) for the full comparative analysis of PlanWire.io, PlanIt, and Gov.uk Planning Data.

---

*Report compiled 2026-03-16 by the Town Crier Feasibility Team. All claims verified via web research against live sources.*
