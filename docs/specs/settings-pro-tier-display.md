# Settings Pro Tier Display Bug

GH: https://github.com/AmyDe/town-crier/issues/353

## Status

Open

## Problem

iOS Settings → Subscription → Current Plan shows **Free** for accounts with an active Pro entitlement in Cosmos. Confirmed on `christy@salter.uk` (prod): Cosmos `Users` doc has `tier="Pro"`, `subscriptionExpiry="2099-12-31"`, but iOS renders Free.

Root cause candidates:
- (A) `POST /v1/me` throws (network / auth / decode) → `ensureServerProfileTier()` returns nil → `SettingsViewModel.fetchServerTier()` falls back to `.free` (hardcoded fallback, not cached tier). JWT claim is also Free (Auth0 `app_metadata` stale or never set). StoreKit is Free (admin/offer-code grant — no App Store purchase).
- (B) `POST /v1/me` returns Pro but iOS DTO decode fails.
- (C) Auth0 user ID mismatch — server returns fresh Free profile under a new sub.

## Fix

### Phase 1 — iOS: shared resolver + no-downgrade fallback (tc-sslz-tier-ios)

Extract a shared `SubscriptionTierResolver` that both `AppCoordinator` and `SettingsViewModel` use, fixing the third recurrence of the Settings ↔ Coordinator drift (tc-aza5).

**Shared resolver** (`SubscriptionTierResolver.swift`, new file in `Coordinators/`):
- Takes `(jwtTier, previousTier, serverFetcher, storeKitFetcher)` → `(SubscriptionTier, isTrialPeriod)`.
- On server-fetch failure: falls back to `max(previousTier, jwtTier)`, **never `.free`** (mirrors `AppCoordinator.swift:223-224`).
- Emits `os.Logger` notice (`subsystem: "uk.towncrierapp"`, `category: "SubscriptionTierResolver"`) when the winner is `.free`, logging all three source values + user `sub`.
- When the winner is `.free` on first pass: calls `authService.refreshSession()` once and re-resolves. Flag guards against looping.

**Wire-up**:
- `AppCoordinator.resolveSubscriptionTier()` → delegates to shared resolver, passes `subscriptionTier` as `previousTier`.
- `SettingsViewModel.resolveSubscriptionTier(jwtTier:)` → delegates to shared resolver, adds `cachedSubscriptionTier` field as `previousTier`.

**Tests** (`SubscriptionTierResolverTests.swift`, new):
- `whenServerReturnsPro_andJwtIsFree_resolvesPro`
- `whenServerFailsAndPreviousTierWasPro_preservesPro` ← regression for this bug
- `whenServerFailsAndPreviousTierWasFreeButJwtIsPro_resolvesPro`
- `whenAllSourcesReturnFree_resolvesFreeAndLogsNotice`
- `whenWinnerIsFree_andRefreshSessionPromotesJwtToPro_secondPassResolvesPro`
- `whenWinnerIsFreeOnBothPasses_doesNotLoop` (assert exactly one `refreshSession()` call)

Update `SettingsViewModelTests` + `AppCoordinatorTierResolutionTests` to inject a `FakeSubscriptionTierResolver` spy.

**Files**:
- `mobile/ios/packages/town-crier-presentation/Sources/Coordinators/SubscriptionTierResolver.swift` (new)
- `mobile/ios/packages/town-crier-presentation/Sources/Coordinators/AppCoordinator.swift`
- `mobile/ios/packages/town-crier-presentation/Sources/Features/Settings/SettingsViewModel.swift`
- `mobile/ios/packages/town-crier-presentation/Sources/Coordinators/ServerTierResolver.swift` (log level notice + error description)
- `mobile/ios/town-crier-tests/Sources/Features/SubscriptionTierResolverTests.swift` (new)
- `mobile/ios/town-crier-tests/Sources/Features/SettingsViewModelTests.swift`
- `mobile/ios/town-crier-tests/Sources/Features/AppCoordinatorTierResolutionTests.swift`
- `mobile/ios/town-crier-tests/Sources/Spies/FakeSubscriptionTierResolver.swift` (new)

### Phase 2 — API: detect and backfill Auth0 metadata tier drift (tc-sslz-tier-api)

When `CreateUserProfileCommandHandler` finds an existing profile whose `Tier` differs from the caller's JWT `subscription_tier` claim, fire-and-forget `auth0Client.UpdateSubscriptionTierAsync` and log the drift. This heals the class of bugs where admin grants wrote Cosmos but the Auth0 management call silently failed.

**Changes**:
- `CreateUserProfileCommand.cs`: add `JwtSubscriptionTier` (nullable string).
- `UserProfileEndpoints.cs`: read `user.FindFirstValue("subscription_tier")`, pass into command.
- `CreateUserProfileCommandHandler.cs`: compare existing `Tier` vs JWT claim; if mismatch, fire-and-forget `IAuth0ManagementClient.UpdateSubscriptionTierAsync` + log drift.
- `CreateUserProfileResult` shape stays as-is.

**Tests** (`CreateUserProfileCommandHandlerAuth0DriftTests.cs`, new):
- `whenExistingProfileTierMatchesJwtClaim_doesNotCallAuth0`
- `whenExistingProfileTierIsProButJwtClaimIsFree_callsUpdateSubscriptionTier`
- `whenAuth0UpdateThrows_handlerStillReturnsResult`

**Files**:
- `api/src/town-crier.application/UserProfiles/CreateUserProfileCommand.cs`
- `api/src/town-crier.application/UserProfiles/CreateUserProfileCommandHandler.cs`
- `api/src/town-crier.web/Endpoints/UserProfileEndpoints.cs`
- `api/tests/town-crier.application.tests/UserProfiles/CreateUserProfileCommandHandlerAuth0DriftTests.cs` (new)
- `api/tests/town-crier.application.tests/UserProfiles/CreateUserProfileCommandHandlerTests.cs` (update construction)

## Related

- tc-a6it: iOS-only signup backfill (POST /v1/me on tier resolution)
- tc-aza5: earlier Settings ↔ Coordinator drift fix
- Spec: `docs/specs/entitlement-framework.md:125-141`
- Spec: `docs/specs/offer-codes.md:222`
