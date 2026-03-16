# 0010. Subscription Entitlement Flow

Date: 2026-03-16

## Status

Accepted

## Context

Town Crier offers three subscription tiers (Free, Personal at £1.99/mo, Pro at £5.99/mo) sold as auto-renewable subscriptions via the App Store. The system needs a trust model for how the iOS app, the Town Crier API, and Apple's infrastructure interact to grant and enforce entitlements.

Two approaches were considered:

1. **Server-validates-with-Apple**: on purchase, the API calls Apple's App Store Server API to verify the transaction before granting entitlements. Adds a synchronous dependency on Apple's API at purchase time.
2. **Verify Apple-signed JWS locally**: on purchase, the app sends the Apple-signed JWS transaction to the API, which cryptographically verifies the signature without calling Apple. Ongoing lifecycle events (renewals, expiry, refunds) are handled by App Store Server Notifications v2.

## Decision

### Trust Model: Verify JWS Locally + Server Notifications

Use **Option 2** — verify the Apple-signed JWS transaction on the API without a round-trip to Apple's servers at purchase time. App Store Server Notifications v2 handle all subsequent lifecycle events.

**Purchase Flow:**

1. User initiates purchase in iOS app via StoreKit 2
2. StoreKit completes the transaction and provides a signed JWS (JSON Web Signature) transaction
3. iOS app sends the JWS transaction to `POST /v1/subscriptions/verify`
4. API verifies the JWS signature against Apple's public certificate chain (no network call to Apple)
5. API extracts subscription product ID, expiry date, and original transaction ID from the verified payload
6. API writes subscription state to the User document in Cosmos DB
7. API returns the updated entitlement state to the iOS app

**Lifecycle Management via Server Notifications v2:**

The API exposes `POST /v1/webhooks/appstore` as the App Store Server Notifications v2 endpoint, configured in App Store Connect.

| Notification Type | Action |
|-------------------|--------|
| `DID_RENEW` | Extend `subscriptionExpiry`, clear any grace period |
| `DID_FAIL_TO_RENEW` | Set `gracePeriodExpiry` (maintain access during Apple's billing retry) |
| `EXPIRED` | Set `subscriptionTier` to `free` |
| `REFUND` | Set `subscriptionTier` to `free` immediately |
| `DID_CHANGE_RENEWAL_INFO` | Update tier if user upgraded/downgraded (takes effect at next renewal) |

### Entitlement State in Cosmos DB

Stored on the User document:

```json
{
  "subscriptionTier": "free | personal | pro",
  "subscriptionExpiry": "2026-04-16T00:00:00Z",
  "originalTransactionId": "1000000123456789",
  "gracePeriodExpiry": null
}
```

**Tier enforcement:** API checks `subscriptionTier` and `subscriptionExpiry` on every tier-gated request. If `subscriptionExpiry` is in the past and `gracePeriodExpiry` is null or also past, the user is treated as `free` regardless of the stored tier.

### Grace Period Handling

During Apple's billing retry window (up to 60 days), the user **silently retains access** — no UI indication of billing issues. This follows Apple's guidance to avoid alarming users during transient payment failures. Access is only revoked when Apple sends `EXPIRED` or the grace period expires.

### Free Trial

A **7-day free trial** is offered for the Personal tier, configured in App Store Connect as an introductory offer. The trial is managed entirely by Apple — StoreKit 2 handles eligibility, and the JWS transaction includes the offer type. The API treats trial subscriptions identically to paid subscriptions (same tier, same entitlements). On trial expiry without conversion, Apple sends `EXPIRED` and the user reverts to free.

### Restore Purchases

iOS calls `Transaction.currentEntitlements` to retrieve all active transactions, sends them to the API for re-verification. This handles device transfers, reinstalls, and recovery from local state loss. The API re-verifies each JWS and updates Cosmos DB state accordingly.

### Not Supported Initially

- **Family Sharing**: not enabled. Simplifies the entitlement model — one transaction per user.
- **Offer codes / promotional offers**: deferred to a future phase.
- **Android / web subscriptions**: iOS only at launch.

## Consequences

- **No Apple API dependency at purchase time.** The JWS verification is purely cryptographic (local certificate validation), so purchases succeed even if Apple's server API is temporarily unavailable. Faster user experience.
- **Server Notifications v2 are the backbone of lifecycle management.** If notifications are missed (Apple outage, endpoint downtime), subscription state could drift. Mitigation: periodic reconciliation job that calls Apple's App Store Server API to verify active subscriptions — added as a future enhancement if needed.
- **Silent grace period** means some users may have access beyond their paid period during billing retry. This is by design — Apple retries billing for up to 60 days, and most billing issues resolve automatically.
- **7-day trial** adds no code complexity — it's an App Store Connect configuration. The API sees it as a normal subscription with an offer type in the JWS payload.
- **Cosmos DB is the single source of truth for entitlements.** The API never calls Apple during request processing. This keeps tier-gated endpoints fast and eliminates an external dependency from the hot path.
