# App Store Subscription Lifecycle

Date: 2026-04-11

## Context

Town Crier sells auto-renewable subscriptions (Personal £1.99/mo, Pro £5.99/mo) via the App Store. ADR-0010 specifies the trust model: verify Apple-signed JWS locally at purchase time, handle lifecycle events via App Store Server Notifications v2. The domain model (`UserProfile`) already has all the subscription methods (`ActivateSubscription`, `RenewSubscription`, `ExpireSubscription`, `EnterGracePeriod`, `LinkOriginalTransactionId`), and the Auth0 sync pattern is proven in `GrantSubscriptionCommandHandler`. What's missing is the two endpoints and the JWS verification infrastructure that serves them both.

References:
- ADR-0010: `docs/adr/0010-subscription-entitlement-flow.md`
- Entitlement framework: `docs/specs/entitlement-framework.md`
- Domain model: `api/src/town-crier.domain/UserProfiles/UserProfile.cs`
- Auth0 sync pattern: `api/src/town-crier.application/Admin/GrantSubscriptionCommandHandler.cs`

## Design

### JWS Verification

Apple signs all transaction and notification payloads as JWS (JSON Web Signature) using ES256 (ECDSA P-256 + SHA-256). The JOSE header contains an `x5c` array — Apple's certificate chain.

Verification steps:
1. Parse the JWS compact serialization (header.payload.signature), base64url-decode each part
2. Extract `x5c` from the JOSE header — array of base64-encoded X.509 certificates
3. Build and validate the certificate chain: `certs[0]` signed by `certs[1]`, etc.
4. Verify the root certificate against the bundled **Apple Root CA - G3** certificate
5. Extract the public key from `certs[0]` (leaf certificate)
6. Verify the JWS signature using ES256 with the leaf's public key
7. Return the decoded payload

Implementation:
- New infrastructure service: `IAppleJwsVerifier` (port) / `AppleJwsVerifier` (adapter)
- Use `System.Security.Cryptography.X509Certificates` for cert chain validation — AOT-safe, no reflection
- Use `ECDsa` for ES256 signature verification
- Bundle Apple Root CA G3 as an embedded resource (download from https://www.apple.com/certificateauthority/)
- `System.Text.Json` source-generated serialization for all decoded payload models

Decoded payload models (all `[JsonSerializable]`):
- `AppleJwsHeader` — `alg`, `x5c` array
- `JWSTransactionDecodedPayload` — `productId`, `transactionId`, `originalTransactionId`, `bundleId`, `purchaseDate`, `expiresDate`, `type`, `inAppOwnershipType`, `environment`, `appAccountToken`, `offerType`, `revocationDate`, `signedDate`
- `JWSRenewalInfoDecodedPayload` — `originalTransactionId`, `autoRenewProductId`, `autoRenewStatus`, `expirationIntent`, `gracePeriodExpiresDate`
- `ResponseBodyV2DecodedPayload` — `notificationType`, `subtype`, `notificationUUID`, `data` (containing `signedTransactionInfo`, `signedRenewalInfo`, `bundleId`, `environment`)

### Product ID to Tier Mapping

Static mapping from App Store product IDs to domain `SubscriptionTier`:

| Product ID | Tier |
|-----------|------|
| `uk.co.towncrier.personal.monthly` | Personal |
| `uk.co.towncrier.pro.monthly` | Pro |

This lives in the domain layer as a static method on a `ProductMapping` class. Unknown product IDs should throw — fail loud rather than silently granting Free.

### Verify Endpoint

`POST /v1/subscriptions/verify` — authenticated (requires valid JWT)

Purpose: iOS app sends the Apple-signed JWS transaction after a StoreKit 2 purchase. The API verifies it and updates the user's subscription state.

Request body:
```json
{
  "signedTransaction": "<JWS compact serialization>"
}
```

Handler (`VerifySubscriptionCommandHandler`):
1. Verify the JWS using `IAppleJwsVerifier`
2. Validate `bundleId` matches expected app bundle ID (reject mismatches)
3. Validate `environment` matches current environment (Sandbox for dev, Production for prod) — or log a warning and proceed (to avoid blocking purchases during testing)
4. Map `productId` to `SubscriptionTier` via `ProductMapping`
5. Look up `UserProfile` by the authenticated user's ID
6. Call `profile.LinkOriginalTransactionId(originalTransactionId)`
7. Call `profile.ActivateSubscription(tier, expiresDate)`
8. Save to Cosmos DB
9. Sync tier to Auth0 via `IAuth0ManagementClient.UpdateSubscriptionTierAsync()`
10. Return the updated entitlement state

Response (200):
```json
{
  "tier": "Personal",
  "subscriptionExpiry": "2026-05-11T00:00:00Z",
  "entitlements": ["StatusChangeAlerts", "DecisionUpdateAlerts", "HourlyDigestEmails"],
  "watchZoneLimit": 3
}
```

Response (400): Invalid or unverifiable JWS
Response (401): No valid JWT

Endpoint mapping: add to a new `SubscriptionEndpoints` group, authenticated, no entitlement gate (all tiers can verify a purchase).

### Webhook Endpoint

`POST /v1/webhooks/appstore` — unauthenticated (Apple cannot send a JWT), but verified via JWS signature

Purpose: Apple sends Server Notifications v2 when subscription lifecycle events occur. The API processes them to keep entitlement state current.

Request body:
```json
{
  "signedPayload": "<JWS compact serialization>"
}
```

Handler (`HandleAppStoreNotificationCommandHandler`):
1. Verify the outer JWS (`signedPayload`) using `IAppleJwsVerifier`
2. Extract `notificationType`, `subtype`, `notificationUUID`
3. **Idempotency check**: look up `notificationUUID` — if already processed, return 200 immediately. Store processed UUIDs in a lightweight Cosmos document (TTL 30 days).
4. Verify the inner JWS (`signedTransactionInfo`) using `IAppleJwsVerifier`
5. Extract `originalTransactionId` and look up the `UserProfile` that has this transaction ID
6. If no profile found, log a warning and return 200 (Apple may send notifications for transactions we haven't verified yet, e.g. sandbox noise)
7. Route by notification type:

| notificationType | subtype | Action |
|-----------------|---------|--------|
| `SUBSCRIBED` | `INITIAL_BUY` | `ActivateSubscription(tier, expiresDate)` |
| `SUBSCRIBED` | `RESUBSCRIBE` | `ActivateSubscription(tier, expiresDate)` |
| `DID_RENEW` | any | `RenewSubscription(newExpiresDate)` |
| `DID_CHANGE_RENEWAL_PREF` | `UPGRADE` | `ActivateSubscription(newTier, expiresDate)` — immediate |
| `DID_CHANGE_RENEWAL_PREF` | `DOWNGRADE` | Log pending downgrade, no state change (takes effect at renewal) |
| `DID_FAIL_TO_RENEW` | `GRACE_PERIOD` | `EnterGracePeriod(gracePeriodExpiresDate)` |
| `DID_FAIL_TO_RENEW` | (none) | `ExpireSubscription()` |
| `EXPIRED` | any | `ExpireSubscription()` |
| `GRACE_PERIOD_EXPIRED` | any | `ExpireSubscription()` |
| `REFUND` | any | `ExpireSubscription()` |
| `REVOKE` | any | `ExpireSubscription()` |
| `OFFER_REDEEMED` | any | `ActivateSubscription(tier, expiresDate)` |
| Other (`TEST`, `PRICE_INCREASE`, `REFUND_DECLINED`, etc.) | any | Log and return 200 |

8. Save updated `UserProfile` to Cosmos DB
9. Sync tier to Auth0 via `IAuth0ManagementClient.UpdateSubscriptionTierAsync()`
10. Record `notificationUUID` as processed
11. Return 200 (empty body) — Apple expects this

Endpoint mapping: add to `SubscriptionEndpoints`, **no authentication** (Apple sends no auth headers — trust is via JWS verification). Must be excluded from the global auth requirement.

### Configuration

Environment variables / app settings:
- `Apple__BundleId` — expected bundle ID (e.g. `uk.co.towncrier.ios`)
- `Apple__Environment` — expected environment (`Sandbox` or `Production`)

The Apple Root CA G3 certificate is bundled as an embedded resource, not configured.

### App Store Connect Setup (Manual)

After the webhook endpoint is deployed, configure the URL in App Store Connect:
- App Store Connect → App → App Information → App Store Server Notifications
- URL: `https://api.towncrierapp.uk/v1/webhooks/appstore`
- Version: V2

## Scope

**In:**
- `IAppleJwsVerifier` / `AppleJwsVerifier` (infrastructure)
- All JWS decoded payload models (infrastructure)
- `ProductMapping` (domain)
- `VerifySubscriptionCommand` / `VerifySubscriptionCommandHandler` (application)
- `HandleAppStoreNotificationCommand` / `HandleAppStoreNotificationCommandHandler` (application)
- `SubscriptionEndpoints` (web)
- Notification idempotency store (infrastructure — Cosmos document with TTL)
- Tests for all of the above

**Out:**
- iOS client changes (separate bead when needed)
- Pulumi infrastructure changes (the endpoint is just a route on the existing Container App)
- Reconciliation job (future enhancement per ADR-0010 — only needed if webhook delivery proves unreliable)
- Family Sharing, offer codes, promotional offers (ADR-0010 explicitly defers these)

## Constraints

- All code must be Native AOT-compatible (no reflection, `System.Text.Json` source generators)
- Follow hexagonal architecture: port interfaces in application, adapters in infrastructure
- Manual CQRS dispatch (no MediatR)
- TDD: red-green-refactor
- `sealed` classes by default
- Cosmos DB SDK directly (no ORM)
