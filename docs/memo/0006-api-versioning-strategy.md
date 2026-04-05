# 0006. API Versioning Strategy

Date: 2026-04-05

## Status

Open

## Question

With web, iOS, and (future) Android clients releasing on different cadences, how should the API handle versioning to maintain compatibility across clients without excessive maintenance burden?

## Analysis

Three axes define a versioning approach:

### 1. Mechanism — how does the client declare which version?

| Mechanism | Pros | Cons |
|-----------|------|------|
| **URL path** (`/v1/applications`) | Visible in logs, easy to route at infra level, trivial to curl/test | URL changes on version bump |
| **Header** (`Api-Version: 2`) | "Pure" REST, URL stability | Invisible in logs/browser, harder to share failing requests, harder to monitor |
| **Query string** (`?api-version=2`) | Easy to add | Messy, pollutes caching, not conventional |

### 2. Granularity — what gets versioned?

| Granularity | Pros | Cons |
|-------------|------|------|
| **Global** (whole API moves together) | Simple mental model: "iOS 2.1 uses API v2" | Forces version bump for all resources even if only one changed |
| **Per-resource** (`/v2/applications` but `/v1/users`) | Surgical, only version what changed | Creates a matrix of versions × resources; hard to reason about which client uses what |

### 3. Compatibility strategy — how are old versions maintained?

| Strategy | Pros | Cons |
|----------|------|------|
| **Additive-only** | Zero cost, no versioning machinery needed | Can't remove fields or change semantics |
| **Side-by-side handlers** | Simple, explicit, each version is self-contained | Some duplication between v1/v2 handlers |
| **Converter chain** (Stripe-style: upversion requests, downversion responses through a pipeline) | Elegant, scales to many versions, single "current" implementation | High complexity, hard to debug, overkill for few clients |

### How CQRS helps

The existing manual CQRS dispatch means versioning maps naturally to the handler layer. A breaking change produces a new handler (e.g. `GetApplicationsV2Query`) returning a different DTO shape, while the domain layer remains unversioned. Since handlers are thin orchestrators, duplicating one for a new version is cheap.

### Scale considerations

Town Crier has 3 controlled clients (web deploys instantly; mobile has app store delays but supports minimum version enforcement). At most 2 API versions would be live concurrently — the current version and the previous one during a migration window. This is fundamentally different from Stripe's problem (hundreds of versions, millions of uncontrolled integrators).

## Options Considered

### A. Additive-only (no formal versioning)

Never introduce breaking changes. Add new optional fields, new endpoints, deprecation headers on old ones. JSON clients that ignore unknown fields (System.Text.Json default behaviour) are naturally forward-compatible.

**Trade-off:** Handles ~80% of API evolution for free, but eventually a breaking change will be necessary (renamed concepts, restructured responses, removed fields).

### B. URL path versioning, global, with side-by-side handlers

When a breaking change is needed, introduce a `/v1/` prefix retroactively on old routes and ship the new shape under `/v2/`. Both prefixes route to different handlers sharing the same domain logic. Sunset the old version after a defined migration window.

**Trade-off:** Simple to implement, debug, and monitor. Some handler duplication, but handlers are thin by design.

### C. Header versioning, per-resource, with converter chain

Each resource versions independently. Clients declare versions via headers. A middleware pipeline transforms requests/responses between versions so only the latest version has a "real" implementation.

**Trade-off:** Maximally flexible and elegant, but operationally complex. Debugging requires knowing the version negotiation path. Per-resource granularity creates a version matrix across clients.

## Recommendation

**Start with A (additive-only), graduate to B (URL path, global, side-by-side) when the first breaking change is unavoidable.**

Concrete approach:

1. **Now**: No version prefix. Evolve the API additively — new optional fields, new endpoints, deprecation headers.
2. **First breaking change**: Add `/v1/` prefix retroactively to existing routes. New shape goes under `/v2/`. Both map to separate handlers calling shared domain logic.
3. **Sunset policy**: Old version is supported for 90 days after the new version ships. Mobile minimum version enforcement accelerates migration.
4. **Concurrency cap**: At most 2 live versions at any time.
5. **No library needed initially**: For 2 concurrent versions with URL path routing, ASP.NET Minimal API route groups with different prefixes are sufficient. `Asp.Versioning` can be adopted later if the versioning surface grows.

Option C (converter chain) is explicitly deferred — it solves a scale problem Town Crier doesn't have. If the client count or integration surface grows dramatically, this memo should be revisited.
