# 0013. Gov.uk Planning Data as Secondary Data Provider

Date: 2026-03-17

## Status

Accepted

## Context

ADR 0006 established PlanIt as the primary data provider for planning applications. However, PlanIt does not include planning designation data — whether a property falls within a conservation area, listed building curtilage, or Article 4 direction area. These designations significantly affect what permitted development rights apply and are important context for users evaluating a planning application's likelihood of approval.

The Gov.uk Planning Data platform (planning.data.gov.uk) is a free, open-data API maintained by DLUHC that provides geospatial designation boundaries for England.

## Decision

We add Gov.uk Planning Data as a secondary data provider, accessed through a new `IDesignationDataProvider` port in the application layer. The `GovUkPlanningDataClient` adapter implements this interface, querying the Gov.uk API with a coordinate pair and returning a `DesignationContext` value object containing:

- Whether the location is within a conservation area (and its name)
- Whether it is within a listed building curtilage (and the grade)
- Whether it is within an Article 4 direction area

The adapter follows the hexagonal architecture established in ADR 0002. It uses `System.Text.Json` with a `[JsonSerializable]` source-generated context for Native AOT compatibility (ADR 0001).

**Graceful degradation:** If the Gov.uk API is unavailable or returns an error, the handler returns `DesignationContext.None` rather than failing the request. Designation data enriches the user experience but is not essential to the core planning application feed.

## Consequences

- **Simpler:** The `IDesignationDataProvider` port establishes a pattern for adding further data sources (e.g., flood risk, environmental designations) without modifying existing application logic.
- **Simpler:** Gov.uk Planning Data is free and open, so there is no cost or API key management overhead.
- **Harder:** Introduces a runtime dependency on a third-party government API. Graceful degradation mitigates this, but prolonged outages would silently degrade the user experience without explicit notification.
- **Harder:** The Gov.uk API has no SLA or rate limit guarantees. If call volume grows significantly, we may need to cache designation boundaries locally rather than querying per-request.
