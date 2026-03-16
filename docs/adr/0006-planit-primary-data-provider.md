# 0006. PlanIt as Primary Data Provider (Polling-Based Ingestion)

Date: 2026-03-16

## Status

Accepted — supersedes [0003](0003-external-data-provider.md)

## Context

ADR 0003 selected PlanWire.io as the primary data provider based on its webhook support. Validation confirmed PlanWire is a real, operational service — but also revealed that PlanWire's `raw` field contains PlanIt (planit.org.uk) scraper data. PlanWire is an API layer built on top of PlanIt's dataset.

Given that:

1. **PlanIt is the upstream source** — PlanWire adds a paid wrapper (£29–299/mo) over data that PlanIt serves for free.
2. **PlanIt has wider coverage** — 417 LPAs and ~20 million applications vs PlanWire's 379 LPAs.
3. **PlanIt is established** — years of operation, open-source scraper ([UKPlanning](https://github.com/aspeakman/UKPlanning)), and downstream consumers (PlanWire itself, the `acton` R package).
4. **Near-real-time latency is unnecessary** — planning applications have consultation periods measured in weeks. A 15–30 minute polling interval is indistinguishable from "instant" for this domain.
5. **The cost saving is meaningful** — eliminating the PlanWire subscription reduces baseline costs by £29–99/mo, which matters at the early stage.

The trade-off is that PlanIt has no webhook support, so Town Crier must implement a polling service with change detection.

## Decision

We will use **PlanIt (planit.org.uk)** as the primary data provider, ingesting planning application data via a **polling service** that periodically queries the PlanIt API and upserts new or changed applications into Cosmos DB.

### Polling Design

- A **background service** (hosted in Azure Container Apps) polls PlanIt on a configurable interval (default: 15 minutes).
- Each poll queries `GET /api/applics/json?different_start={last_poll_iso}&pg_sz=5000&sort=-last_different` to retrieve applications whose **content** changed since the last successful poll.
- Uses `different_start` (not `changed_start`) — the `last_changed` timestamp updates on every scrape cycle (~hourly) even when content is unchanged, while `last_different` only updates when application data actually changes. This avoids unnecessary upserts.
- The `name` field (`{area_name}/{uid}`, e.g. `Leeds/26/01471/TR`) is the globally unique idempotency key. The `uid` field is unique within an authority but not globally.
- Results are diffed against existing records in Cosmos DB and upserted.
- Watch zone matching runs on each upsert, unchanged from the original Phase 1 design.

### Data Licensing

PlanIt's data is sourced from UK council planning portals — public information published under statutory obligation. The licensing chain:

| Data Type | Licence | Commercial Use |
|-----------|---------|----------------|
| Planning application metadata (addresses, descriptions, statuses, dates) | Public information from council portals | Permitted |
| Postcode data (via Postcodes.io / Open Postcode Geo) | Open Government Licence + OS OpenData Licence | Permitted with attribution |
| Boundary data (via MapIt / OpenStreetMap) | OGL + Open Database Licence (ODbL) | Permitted with attribution |
| Planning documents (drawings, architectural plans) | Copyright, Designs and Patents Act 1988 s.47 | **Not permitted** — consultation use only |

Town Crier serves application metadata and deep-links to council portals for documents. It does not reproduce copyrighted planning documents. Building a commercial service on top of OGL-licensed public planning data is legally permissible.

### Attribution Requirements (Mandatory)

Town Crier **must** display data attribution in the app and on any public-facing surface (e.g., marketing site). This is both a licence obligation (OGL, ODbL) and the right thing to do for a free, community-run service. Required attributions:

1. **PlanIt** — "Planning data sourced from [PlanIt](https://www.planit.org.uk)" — visible in the app's About/Settings screen and on any screen displaying planning application data (e.g., a subtle footer on map and list views).
2. **Crown Copyright (OGL)** — "Contains public sector information licensed under the Open Government Licence v3.0."
3. **Ordnance Survey** — "Contains OS data © Crown copyright and database right [year]."
4. **OpenStreetMap** — "Contains data from OpenStreetMap © OpenStreetMap contributors, ODbL."

These attributions are non-negotiable and must be implemented before any public release.

### PlanIt API — No Published Terms of Service

PlanIt's API documentation does not explicitly prohibit or permit commercial use. The API is unauthenticated and rate-limited by IP, with unpublished thresholds (429 responses). There is no terms of service page.

**Mitigations:**

- Poll conservatively — respect rate limits, use date filters to minimise request volume, back off on 429 responses.
- Cache aggressively — serve all user-facing reads from Cosmos DB, never proxy PlanIt in real time.
- **Attribute PlanIt visibly** — see Attribution Requirements below.
- Contact Andrew Speakman (PlanIt maintainer) once we have a working MVP to introduce the project, confirm acceptable use, and offer a donation or attribution arrangement.

### API Validation Results (2026-03-16)

Live trial calls were made against the PlanIt API to confirm compatibility with the polling design. All critical endpoints are operational.

#### Endpoints Tested

| Test | Query | Result |
|------|-------|--------|
| Applications by authority + changed | `?auth=Leeds&changed=1&pg_sz=3` | **Pass** — 1,538 results, 0.55s |
| Date-range change detection | `?auth=Leeds&changed_start=2026-03-15&changed_end=2026-03-16` | **Pass** — correct filtering, 0.17s |
| Spatial search (postcode + radius) | `?pcode=LS1+1UR&krad=1&recent=7` | **Pass** — 3 results with `distance` field, 3.6s |
| Field projection (`select`) | `?select=uid,address,description,app_type,...` | **Pass** — returns only requested fields |
| Pagination | `?auth=Leeds&changed=1&pg_sz=2&page=2` | **Pass** — `from: 2`, correct offset |
| Active areas list | `/api/areas/json?area_type=active` | **Pass** — **417 LPAs** confirmed |
| Areas with field projection | `?select=area_name,long_name,area_type,total` | **Pass** — field names use snake_case |

#### Response Structure (Application Record)

Top-level fields available on every record:

| Field | Example | Notes |
|-------|---------|-------|
| `uid` | `26/01471/TR` | Unique within authority |
| `name` | `Leeds/26/01471/TR` | Globally unique (`{area}/{uid}`) |
| `address` | `Highgate House Grove Lane...` | Free text |
| `postcode` | `LS6 2AP` | Extracted postcode |
| `description` | `T1 lime tree...` | Full description text |
| `app_type` | `Trees`, `Full`, `Heritage`, `Amendment` | Application category |
| `app_state` | `Undecided`, `Permitted`, `Refused` | Decision status |
| `app_size` | `Small`, `Large` | Development scale |
| `start_date` | `2026-03-13` | Date received/validated |
| `decided_date` | `null` or `YYYY-MM-DD` | Decision date |
| `consulted_date` | `null` or `YYYY-MM-DD` | Consultation end date |
| `area_name` | `Leeds` | Planning authority name |
| `area_id` | `292` | PlanIt authority ID |
| `location` | `{"type":"Point","coordinates":[-1.577,53.824]}` | GeoJSON point |
| `location_x` / `location_y` | `-1.577373` / `53.824035` | Flat lng/lat |
| `url` | `https://publicaccess.leeds.gov.uk/...` | Council portal deep link |
| `link` | `https://www.planit.org.uk/planapplic/...` | PlanIt detail page |
| `last_changed` | `2026-03-16T08:09:03.598` | Updated every scrape cycle |
| `last_different` | `2026-03-14T11:59:17.642` | Updated only on content change |
| `last_scraped` | `2026-03-14T11:59:17.642` | Last scrape timestamp |
| `other_fields` | `{...}` | Semi-structured, council-specific |

The `other_fields` object varies by council/scraper type and may include: `ward_name`, `parish`, `agent_name`, `applicant_name`, `date_received`, `date_validated`, `target_decision_date`, `comment_url`, `docs_url`, `n_documents`, `easting`/`northing`. Treat as semi-structured — do not depend on any field being present.

#### Design Implications

1. **No geocoding needed** — GeoJSON `location` and flat `location_x`/`location_y` are included on 91% of records. Spatial watch-zone matching can use these directly.
2. **`select` parameter is slower** — queries with field projection took up to 10s vs 0.17–0.55s without. The poller should fetch full records rather than projecting.
3. **Spatial queries are available but slower** — postcode + radius queries (1.5–3.6s) are useful for initial zone backfill but not for polling. Polling should use date-range filters only.
4. **5,000 results/page confirmed** — at a 15-minute polling interval, change volumes should be well under this limit. A single request per poll cycle is sufficient.
5. **`select` field names are snake_case** — the API returns 400 if field names don't match exactly (e.g. `area_name` not `name` on areas). Discovered through trial and error.

### Provider Abstraction

The `IPlanningDataProvider` port interface remains critical. The adapter-based architecture allows:

- Swapping PlanIt for PlanWire (or another provider) without changing application logic.
- Adding Gov.uk Planning Data as a supplementary source for boundary/designation data.
- Running integration tests against a mock provider (ADR 0004 scope changes from mocking PlanWire to mocking PlanIt, but the principle is identical).

## Consequences

### What becomes easier

- **Lower baseline cost** — £0 data provider cost vs £29–99/mo for PlanWire. Baseline drops from ~£40–55/mo to ~£17–32/mo.
- **No API key management** — PlanIt requires no authentication. Simpler secrets management, easier contributor onboarding.
- **Better coverage** — 417 LPAs vs 379. No coverage gaps to fill.
- **Better backfill** — 5,000 results/page vs PlanWire's 100. One-time zone backfill is faster and cheaper.
- **No vendor lock-in to an unverified service** — PlanWire has no third-party reviews, no published ToS, and an unknown operating entity.

### What becomes harder

- **Polling service complexity** — must build and maintain a polling scheduler with change detection, deduplication, and error handling. This is a one-time development cost, not ongoing.
- **Notification latency** — 15–30 minutes vs PlanWire's near-real-time webhooks. Acceptable for planning applications (consultation periods are weeks).
- **Rate limit discovery** — PlanIt's rate limits are unpublished. Must be discovered experimentally and respected conservatively.
- **Single-maintainer risk** — PlanIt is run by one developer. Mitigated by the adapter architecture (can swap providers) and by the fact that PlanWire itself depends on PlanIt, so if PlanIt goes down, PlanWire likely does too.
- **ADR 0004 scope change** — the mock service now targets PlanIt's API contract instead of PlanWire's.
