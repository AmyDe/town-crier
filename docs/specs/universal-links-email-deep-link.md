# Universal Links — Email Card Deep-Links

GH: https://github.com/AmyDe/town-crier/issues/362

## Status

Open

## Problem

Digest email cards show plain-text addresses. Gmail's auto-linker turns them into Google Maps links, pulling users out of Town Crier into Maps on tap. Every email tap should open the Town Crier app (if installed) or the web detail page (if not).

## Goal

Wrap each notification card in `<a href="https://towncrierapp.uk/applications/{uid}">` so iOS's Universal Links mechanism intercepts the URL and opens `ApplicationDetailView`. Non-iOS / app-not-installed: opens the web app at the same URL (requires auth return-URL fix so the user lands on the detail page, not `/dashboard`).

## Key Technical Context

- **iOS deep-link plumbing already exists**: `DeepLink.applicationDetail(PlanningApplicationId)` + `AppCoordinator.handleDeepLink` (`:366`). Push notifications use this path today — Universal Links reuse it.
- **Web route already exists**: `AppRoutes.tsx:67` has `/applications/*` → `ConnectedApplicationDetailPage`; UID extracted via `useParams()['*']` for slash-containing PlanIt UIDs.
- **iOS identifiers**: bundle ID `uk.towncrierapp.mobile`, team ID `4574VQ7N2X`.
- **AASA scope**: claim only `/applications` and `/applications/*` — avoid hijacking other paths.
- **Gmail iOS in-app browser** does not trigger Universal Links (opens web view) — acceptable now; deferred Smart App Banner when App Store ID is known.
- **Email wrapping**: wrap each `<div>` individually (not `<tr>`/`<td>`) for Outlook compatibility.
- **Dead code**: `SendNotificationAsync` / `BuildNotificationHtml` have zero callers — do not touch (separate cleanup bead).

## Pre-Resolved Decisions

- Universal Links (not custom URL schemes): gives app-if-installed / web-otherwise from one href.
- Entire card clickable (all three divs), not just the address — large tap target + suppresses Gmail Maps detection.
- AASA narrow scope: `/applications` + `/applications/*` only.
- Bottom CTA changes from `https://towncrierapp.uk` → `https://towncrierapp.uk/applications`.
- Single PR ships all three layers (owner directive: no real users, simplicity > phased deploy).
- Smart App Banner deferred (no real App Store ID yet — placeholder `id000000000`).

---

## Phase 1 — API: Clickable Email Cards

**Bead: tc-univlink-api**

`api/src/town-crier.infrastructure/Notifications/AcsEmailSender.cs`:
- `BuildNotificationCard`: wrap each of the three `<div>` lines in `<a href="{appUrl}" style="text-decoration:none;color:inherit;">…</a>`. App URL: `https://towncrierapp.uk/applications/{ApplicationId}` (percent-encode each path segment for slash-containing UIDs).
- `BuildDigestHtml`: change bottom CTA href from `https://towncrierapp.uk` → `https://towncrierapp.uk/applications`.

Tests (`api/tests/town-crier.infrastructure.tests/Notifications/AcsEmailSenderTests.cs`, create if missing):
- `buildDigestHtml_cardContainsApplicationDetailLink`
- `buildDigestHtml_bottomCtaPointsToApplicationsList`
- `buildDigestHtml_uidWithSlashesIsUrlEncoded`

**Files:**
- `api/src/town-crier.infrastructure/Notifications/AcsEmailSender.cs`
- `api/tests/town-crier.infrastructure.tests/Notifications/AcsEmailSenderTests.cs`

---

## Phase 2 — Web: AASA File + Auth Return-URL

**Bead: tc-univlink-web**

`web/public/.well-known/apple-app-site-association` (new, no extension):
```json
{
  "applinks": {
    "details": [
      {
        "appIDs": ["4574VQ7N2X.uk.towncrierapp.mobile"],
        "components": [
          { "/": "/applications" },
          { "/": "/applications/*" }
        ]
      }
    ]
  }
}
```

`web/staticwebapp.config.json` (create or modify): add route rule serving `/.well-known/apple-app-site-association` with `Content-Type: application/json`. Apple rejects any other MIME type.

`web/src/auth/AuthGuard.tsx`: call `loginWithRedirect({ appState: { returnTo: window.location.pathname + window.location.search } })`.

`web/src/auth/CallbackPage.tsx`: after Auth0 callback, read `appState.returnTo`; if present, `<Navigate to={returnTo} replace />`; otherwise default to `/dashboard`.

Tests (Vitest):
- `web/src/auth/__tests__/AuthGuard.test.tsx`: assert `loginWithRedirect` called with `appState.returnTo` = current pathname.
- `web/src/auth/__tests__/CallbackPage.test.tsx`: assert navigation to `returnTo` when present; `/dashboard` fallback.

**Files:**
- `web/public/.well-known/apple-app-site-association`
- `web/staticwebapp.config.json`
- `web/src/auth/AuthGuard.tsx`
- `web/src/auth/CallbackPage.tsx`
- `web/src/auth/__tests__/AuthGuard.test.tsx`
- `web/src/auth/__tests__/CallbackPage.test.tsx`

---

## Phase 3 — iOS: Universal Links Entitlement + Handler

**Bead: tc-univlink-ios** (depends on Phase 2 — AASA must be deployed for UL to work in prod)

`mobile/ios/town-crier-app/TownCrierApp.entitlements`:
```xml
<key>com.apple.developer.associated-domains</key>
<array>
  <string>applinks:towncrierapp.uk</string>
</array>
```

`mobile/ios/packages/town-crier-presentation/Sources/Coordinators/UniversalLinkParser.swift` (new):
- Pure helper, sits alongside `NotificationPayloadParser.swift`.
- `parse(_ url: URL) -> DeepLink?` — returns `.applicationDetail(PlanningApplicationId(path))` for `/applications/{uid}`, or a new `.applicationsTab` case (or existing tab-switch signal) for `/applications` exactly, `nil` otherwise.

`mobile/ios/town-crier-app/Sources/TownCrierApp.swift`:
- Add `.onContinueUserActivity(NSUserActivityTypeBrowsingWeb) { activity in … }` alongside existing `.onOpenURL`.
- Extract `activity.webpageURL`, pass to `UniversalLinkParser.parse()`, call `coordinator.handleDeepLink(_:)` or `coordinator.selectedTab = .applications` as appropriate. Fall through (do nothing) on `nil`.

Tests (`mobile/ios/town-crier-tests/Sources/Coordinators/UniversalLinkParserTests.swift`, Swift Testing):
- `/applications/19/00123/FUL` → `.applicationDetail("19/00123/FUL")`
- `/applications` → applications-tab signal
- `/foo` → `nil` (no-op)
- Empty path → `nil`

**Files:**
- `mobile/ios/town-crier-app/TownCrierApp.entitlements`
- `mobile/ios/town-crier-app/Sources/TownCrierApp.swift`
- `mobile/ios/packages/town-crier-presentation/Sources/Coordinators/UniversalLinkParser.swift`
- `mobile/ios/town-crier-tests/Sources/Coordinators/UniversalLinkParserTests.swift`

---

## Out of Scope

- Removing dead `SendNotificationAsync` / `BuildNotificationHtml` — separate cleanup bead.
- Smart App Banner — deferred until App Store ID known.
- Push notification deep-linking — already works.
- Public read-only application detail view for unauthenticated users.
- Click-through analytics / UTM parameters.
- Maps integration on detail pages.

## References

- iOS deep-link infra: `AppCoordinator.swift:366`, `DeepLink.swift`, `NotificationPayloadParser.swift`
- Web routing: `AppRoutes.tsx:67`, `ApplicationDetailPage.tsx:23`
- Email rendering: `AcsEmailSender.cs:89` (`BuildDigestHtml`), `:146` (`BuildNotificationCard`)
