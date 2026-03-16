# ADR 0003: External Data Provider Selection

## Status
Proposed

## Context
The "town-crier" application requires a comprehensive and reliable source of UK local authority planning application data. Manually scraping 379+ individual council portals is technically complex, resource-intensive, and difficult to maintain over time. We need a provider that offers high coverage, low latency for new applications, and a cost-effective entry point.

## Decision
We will use **Planwire.io** as our primary external data provider for sourcing planning application data across the UK.

## Rationale
- **Comprehensive Coverage:** Supports 379 Local Planning Authorities (LPAs) across the UK, providing near-total national coverage from a single integration point.
- **Webhook Integration:** Offers native webhook support for `application.new` and `application.updated` events. This is critical for our push-notification-first architecture, as it allows for reactive data ingestion rather than expensive polling.
- **Cost-Effective Scaling:** 
    - A **Free Tier** (100 requests/day) allows for development and initial prototyping at $0 cost.
    - A **Starter Tier** (£29/mo for 1,000 requests/day) provides a clear and affordable path to production as the user base grows.
- **Rich Data & Search:** Provides normalized JSON data including addresses, geospatial coordinates (lat/lng), descriptions, and status updates, which simplifies our backend logic.
- **Developer-Friendly:** Standard REST API with HMAC-signed webhooks ensures secure and straightforward integration with our .NET 10 backend.

## Consequences
- **Third-Party Dependency:** The application's core functionality depends on Planwire's uptime and data accuracy.
- **Subscription Costs:** As the user base grows, the project will need to transition to paid tiers (£29/mo+), necessitating a monetization strategy (e.g., a low-cost subscription model) to remain sustainable.
- **Data Caching:** To minimize API request costs and stay within daily limits, the backend will implement a caching strategy in Cosmos DB for frequently accessed application data.
- **Webhook Security:** The .NET 10 API must implement HMAC-SHA256 signature verification to ensure the authenticity of incoming webhooks.
