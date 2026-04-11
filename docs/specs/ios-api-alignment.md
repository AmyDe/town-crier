# iOS API Alignment & Entitlement Gating

Date: 2026-04-11

## Context

The web app has reached feature-complete status against the full API surface (31 endpoints, 78 tests, 13 features). The iOS app was built earlier and has fallen behind: it wires only 7 of 25+ endpoints, has stale DTOs, and determines subscription tier locally via StoreKit rather than the JWT-based entitlement framework the API enforces.

This spec covers the work needed to bring the iOS app into alignment with the current API surface, fix DTO/model drift, and implement the JWT-based entitlement gating pattern so the iOS app gates features consistently with the web and API.

References:
- Feature list: `docs/specs/feature-list.md`
- Entitlement framework: `docs/specs/entitlement-framework.md`
- Hourly email digest: `docs/specs/hourly-email-digest.md`
- iOS offline architecture: ADR-0014
- Auth0 authentication: ADR-0007

## Phase 1: Fix API Drift (Foundation)

Correct existing API calls that have diverged from the current API contract. These must land first because every subsequent feature depends on correct DTOs and status mapping.

### 1.1 Fix ApplicationStatus enum

The iOS `ApplicationStatus` enum uses `underReview` which no longer matches the API. The API sends `appState` values:

| API value | Current iOS case | Correct iOS case |
|-----------|-----------------|-----------------|
| `Undecided` | (missing, falls to `.unknown`) | `.undecided` |
| `Approved` | `.approved` | `.approved` (no change) |
| `Refused` | `.refused` | `.refused` (no change) |
| `Withdrawn` | `.withdrawn` | `.withdrawn` (no change) |
| `Appealed` | `.appealed` | `.appealed` (no change) |
| `Not Available` | (missing, falls to `.unknown`) | `.notAvailable` |
| `Under Review` | `.underReview` | remove |

Changes:
- `ApplicationStatus.swift`: rename `.underReview` to `.undecided`, add `.notAvailable`
- `APIPlanningApplicationRepository.swift` `mapAppState`: map `"Undecided"` to `.undecided`, `"Not Available"` to `.notAvailable`, remove `"Under Review"` mapping
- `synthesizeStatusHistory`: replace `.underReview` references with `.undecided`
- `ApplicationStatus+Display.swift`: update display labels/colours for new cases
- Update all tests referencing `.underReview`

### 1.2 Fix CreateWatchZoneRequest

The iOS `CreateWatchZoneRequest` sends a client-generated `zoneId`. The API does not accept this field — it generates the ID server-side and returns it in the `Location` header.

Changes:
- `APIWatchZoneRepository.swift`: remove `zoneId` from `CreateWatchZoneRequest`, add optional `authorityId` field
- `CreateWatchZoneRequest`: fields become `name`, `latitude`, `longitude`, `radiusMetres`, `authorityId`
- `save(_:)` method: parse the `Location` header or `201 Created` response to extract the server-assigned zone ID. This requires extending `URLSessionAPIClient` to return response metadata (or adding a separate method for POST-with-location)
- `WatchZone` domain model: consider whether `id` should be optional pre-save, or use a factory pattern where `save()` returns the created zone

### 1.3 Fix WatchZone domain model

The `WatchZoneSummaryDTO.toDomain()` drops `authorityId` during mapping. This field is needed for fetching applications by authority.

Changes:
- `WatchZone`: add `authorityId: Int` property
- `WatchZoneSummaryDTO.toDomain()`: pass through `authorityId`
- `WatchZoneLimits`: update `maxZones` — currently `personal` has `maxZones = 1` but API `EntitlementMap` gives Personal 3 zones. Correct to: Free = 1, Personal = 3, Pro = unlimited

### 1.4 Fix UserProfile to include tier

The iOS `UserProfile` is a thin value object (userId, email, name). The API's `GET /v1/me` returns `tier`, `pushEnabled`, `digestDay`, `emailDigestEnabled`. The iOS app needs this server-side profile.

Changes:
- New domain type: `ServerProfile` (or extend `UserProfile`) with: `userId`, `tier: SubscriptionTier`, `pushEnabled: Bool`, `digestDay: DayOfWeek`, `emailDigestEnabled: Bool`
- New protocol: `UserProfileRepository` with `create()`, `fetch()`, `update(...)`, `delete()`, `exportData()`
- New data adapter: `APIUserProfileRepository` wiring:
  - `POST /v1/me` (create, no body — API reads JWT claims)
  - `GET /v1/me` (fetch)
  - `PATCH /v1/me` (update: `pushEnabled`, `digestDay`, `emailDigestEnabled`)
  - `DELETE /v1/me` (delete account — cascade)
  - `GET /v1/me/data` (export)

## Phase 2: Entitlement Gating

Wire the JWT-based entitlement system so the iOS app shows/hides features and redirects to subscription upsell consistently with the API and web app.

### 2.1 Extract subscription_tier from JWT

The `Auth0AuthenticationService` currently decodes the ID token for email/name but never reads the access token. The `subscription_tier` custom claim is on the **access token** (added by the Auth0 Post-Login Action).

Changes:
- `Auth0AuthenticationService.mapToSession()`: decode the **access token** and extract `subscription_tier` claim (default to `"Free"` if absent)
- `AuthSession`: add `subscriptionTier: SubscriptionTier` property
- This becomes the single source of truth for what the user can do in the current session

### 2.2 Entitlement model (domain layer)

Mirror the API's entitlement model in the iOS domain layer so gating logic is consistent.

New types:
```swift
enum Entitlement: String, CaseIterable, Sendable {
    case searchApplications
    case statusChangeAlerts
    case decisionUpdateAlerts
    case hourlyDigestEmails
}

enum Quota: Sendable {
    case watchZones
}

struct EntitlementMap {
    static func entitlements(for tier: SubscriptionTier) -> Set<Entitlement>
    static func limit(for tier: SubscriptionTier, quota: Quota) -> Int
}
```

Mapping (must match API's `EntitlementMap.cs`):
- **Free**: no entitlements, 1 watch zone
- **Personal**: statusChangeAlerts, decisionUpdateAlerts, hourlyDigestEmails; 3 watch zones
- **Pro**: all of the above + searchApplications; unlimited watch zones

Retire `WatchZoneLimits` or have it delegate to `EntitlementMap.limit(for:quota:)`.

### 2.3 Proactive UI gating

Mobile UX should gate **proactively** (disable/badge before the user taps) unlike the web which gates reactively (attempt call, show gate on 403). This avoids a network round-trip to discover you can't do something.

Pattern: ViewModels receive `subscriptionTier` (from `AuthSession`) and use `EntitlementMap` to decide what to show.

| Feature | Entitlement / Quota | Free behaviour | Personal+ behaviour |
|---------|-------------------|----------------|-------------------|
| Add watch zone | `Quota.watchZones` | Disabled + "Upgrade" badge after 1 zone | Enabled (up to tier limit) |
| Search | `Entitlement.searchApplications` | Tap shows subscription upsell sheet | Pro only — enabled |
| Zone notification prefs (status changes) | `Entitlement.statusChangeAlerts` | Toggle disabled + upgrade prompt | Enabled |
| Zone notification prefs (decision updates) | `Entitlement.decisionUpdateAlerts` | Toggle disabled + upgrade prompt | Enabled |
| Saved applications | none | Available to all | Available to all |
| Notifications list | none | Available to all | Available to all |
| Map | none | Available to all | Available to all |
| Dashboard | none | Available to all | Available to all |

### 2.4 Reactive 403 fallback

Even with proactive gating, the API may return 403 if the local tier is stale (e.g. subscription expired between token refreshes).

Changes:
- New `DomainError` case: `.insufficientEntitlement(required: String)`
- `URLSessionAPIClient`: detect 403 responses with `error: "insufficient_entitlement"`, parse the `required` field, throw `.insufficientEntitlement(required:)`
- ViewModels catch `.insufficientEntitlement` and present the subscription upsell sheet
- This is a safety net, not the primary UX — proactive gating should prevent most 403s

### 2.5 Subscription upsell sheet

A reusable SwiftUI sheet presented when a user attempts an action above their tier.

Design:
- Title: "Upgrade to unlock"
- Body: describes the specific feature (parameterised by entitlement name)
- CTA: "View Plans" — navigates to existing `SubscriptionView`
- Secondary: "Not now" — dismisses
- Presented via `.sheet(item:)` binding on a new `@Published var entitlementGate: Entitlement?` on ViewModels that need it

### 2.6 Post-purchase token refresh

After a successful StoreKit purchase, the iOS app must refresh the Auth0 token so the `subscription_tier` claim reflects the new tier.

Flow:
1. `SubscriptionViewModel.purchase()` succeeds via StoreKit
2. Call API subscription verification endpoint (if/when it exists — see phase 4)
3. Call `authService.refreshSession()` to get a new token with updated `subscription_tier`
4. Update `AuthSession.subscriptionTier` from the refreshed token
5. All observing ViewModels react to the new tier and update their UI

Interim (before API verification endpoint): continue using StoreKit-local tier for immediate UX feedback, but refresh the token so API calls succeed. The API's tier will update when the admin grants it or the verification endpoint is built.

## Phase 3: Missing Feature Endpoints

Wire the API endpoints that the iOS app doesn't call yet. Each endpoint follows the established port/adapter pattern.

### 3.1 User profile lifecycle

New `UserProfileRepository` protocol and `APIUserProfileRepository` adapter (see phase 1.4 for detail).

Integration points:
- **Onboarding**: call `POST /v1/me` after Auth0 login succeeds, before creating the first watch zone
- **Settings**: call `GET /v1/me` to display tier and preferences; `PATCH /v1/me` to save preference changes; `DELETE /v1/me` for account deletion (currently calls Auth0 only — must also hit API for Cosmos cascade)
- **App launch**: call `GET /v1/me` to hydrate tier and preferences; 404 = redirect to onboarding

### 3.2 Application authorities

New endpoint: `GET /v1/me/application-authorities`

Response shape:
```json
{
  "authorities": [{ "id": 123, "name": "Bath and NE Somerset", "areaType": "..." }],
  "count": 2
}
```

Integration: Replace the current pattern where the user must know their authority. The applications tab should show a list of the user's authorities (derived from watch zones) and let them tap to browse.

### 3.3 Saved applications

New protocol: `SavedApplicationRepository`
- `save(applicationUid: String) async throws`
- `remove(applicationUid: String) async throws`
- `loadAll() async throws -> [SavedApplication]`

New adapter: `APISavedApplicationRepository` wiring:
- `PUT /v1/me/saved-applications/{uid}` (no body, 204)
- `DELETE /v1/me/saved-applications/{uid}` (204)
- `GET /v1/me/saved-applications` (returns array with `applicationUid`, `savedAt`, nested `application` object)

Note: the `{uid}` path parameter uses a greedy match (`**`) on the API because PlanIt UIDs contain slashes. The iOS client must URL-encode the UID or pass it as-is (the API handles it).

Integration: Save/unsave button on `ApplicationDetailView` and `MapView` application summary sheet. New `SavedApplicationsView` + `SavedApplicationsViewModel` for the list.

### 3.4 Notifications list

New protocol: `NotificationRepository`
- `fetch(page: Int, pageSize: Int) async throws -> NotificationPage`

Response shape:
```json
{
  "notifications": [{
    "applicationName": "...",
    "applicationAddress": "...",
    "applicationDescription": "...",
    "applicationType": "...",
    "authorityId": 123,
    "createdAt": "2026-04-10T14:30:00Z"
  }],
  "total": 42,
  "page": 1
}
```

New `NotificationsView` + `NotificationsViewModel` with pagination (same pattern as web's `usePaginatedFetch`).

### 3.5 Search (Pro-gated)

New protocol: `SearchRepository`
- `search(query: String, authorityId: Int, page: Int) async throws -> SearchResult`

Gated by `Entitlement.searchApplications` — proactively show subscription upsell for Free users, reactively handle 403 as fallback.

Response shape:
```json
{
  "applications": [{ "uid": "...", "name": "...", "address": "...", ... }],
  "total": 100,
  "page": 1
}
```

New `SearchView` + `SearchViewModel`. Includes authority selector (reuse `GET /v1/authorities?search=` endpoint) and paginated results.

### 3.6 Dashboard

New `DashboardView` + `DashboardViewModel` combining:
- Watch zones summary (from existing `GET /v1/me/watch-zones`)
- Application authorities (from new `GET /v1/me/application-authorities`)
- Quick links to Saved, Notifications, Map

This becomes the post-onboarding landing screen, matching the web's dashboard.

### 3.7 Per-zone notification preferences

Endpoints:
- `GET /v1/me/watch-zones/{zoneId}/preferences`
- `PUT /v1/me/watch-zones/{zoneId}/preferences` (entitlement-gated — needs Personal+ for status/decision toggles)

Response/request shape:
```json
{
  "zoneId": "...",
  "newApplications": true,
  "statusChanges": false,
  "decisionUpdates": false
}
```

Integration: `WatchZoneEditView` — per-zone toggles for notification types, with entitlement gating on the paid toggles.

## Phase 4: Future API Work (Out of Scope)

These items require API-side changes and are tracked separately:

- **Subscription verification endpoint** (`POST /v1/subscriptions/verify`): receives StoreKit JWS, verifies with Apple, updates Cosmos + Auth0. Until this exists, tier sync from iOS purchases requires admin intervention.
- **Legal documents from API** (`GET /v1/legal/{type}`): replace hardcoded legal content in `LegalDocumentViewModel`. Low priority.
- **Designations lookup** (`GET /v1/designations?lat=&lon=`): heritage/conservation area enrichment on application detail. Low priority.
- **Demo account** (`GET /v1/demo-account`): marketing demo mode. Low priority.

## Constraints

- All code must follow iOS coding standards skill (MVVM-C, Swift Concurrency, protocol-oriented, `final` by default)
- All new code needs tests (TDD: red-green-refactor)
- Domain layer must remain dependency-free (no Auth0, no Foundation networking)
- Data layer adapters implement domain protocols
- Use existing `URLSessionAPIClient` for all API calls
- Design system: use `TCDesignSystem` tokens for all new UI (colours, typography, spacing)
- Native AOT compatibility is API-side only; iOS has no constraint here
