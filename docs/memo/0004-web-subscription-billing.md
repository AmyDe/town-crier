# 0004. Web Subscription Billing — Stripe vs RevenueCat + Stripe

Date: 2026-04-03

## Status

Open

## Question

How should Town Crier implement web-based subscriptions so that subscribers get access to premium features on all platforms (iOS and web), regardless of where they subscribe?

Currently, premium subscriptions are sold exclusively through Apple In-App Purchase on iOS. Adding a web subscription channel introduces a cross-platform entitlement problem: a user who subscribes on the web must be recognised as premium in the iOS app, and vice versa.

## Analysis

Two viable architectures were evaluated. Both use Stripe as the web payment processor — it is the industry standard for SaaS/digital subscription billing, with first-class support for recurring payments, SCA/3D Secure, dunning, and a customer portal.

The key difference is **who owns the entitlement layer** — do we build it ourselves on top of Stripe and Apple, or delegate it to RevenueCat?

### Option A: Stripe Only (Build Entitlement Layer Ourselves)

Stripe handles web checkout, billing, and subscription lifecycle. We build a unified entitlement service in our API that reconciles subscription state from two sources: Stripe (web) and App Store Server Notifications (iOS).

**Architecture:**

```
iOS App  ──▶  App Store IAP  ──▶  App Store Server Notifications  ──▶  Our API
Web App  ──▶  Stripe Checkout ──▶  Stripe Webhooks                 ──▶  Our API
                                                                         │
                                                              Entitlement Service
                                                              (Cosmos DB record)
                                                                         │
                                                              ◀── All clients query
```

**What we build:**

- Stripe Checkout integration (web) — session creation, success/cancel redirects
- Stripe webhook handler — `customer.subscription.created`, `updated`, `deleted`, `invoice.payment_failed`, etc.
- App Store Server Notifications v2 handler — `DID_RENEW`, `DID_CHANGE_RENEWAL_STATUS`, `EXPIRED`, `REFUND`, `REVOKE`, etc.
- Unified entitlement record in Cosmos DB — single document per user with `{ source: "stripe" | "apple", status, expiresAt, tier }`
- Entitlement query endpoint consumed by both iOS and web clients
- Grace period, retry, and expiry logic for both billing systems
- User identity linking (ensuring Apple user and web user map to the same account)

**Pros:**

- Full control over the billing and entitlement experience
- No additional vendor dependency or revenue share beyond payment processing
- Lower per-transaction cost: Stripe charges 1.5% + 20p (UK cards) or 2.9% + 30p (international)
- Direct access to Stripe's full API — coupons, trials, usage-based billing, invoicing
- Can evolve billing logic freely (e.g., team plans, annual discounts, promotional pricing)

**Cons:**

- Significant engineering effort to build the entitlement reconciliation layer correctly
- Must handle edge cases across two billing systems: overlapping subscriptions, refunds on one platform, upgrades/downgrades, currency differences, grace periods
- Must implement and maintain App Store Server Notifications v2 parsing and verification (JWS signed payloads)
- Tax compliance is our responsibility — need Stripe Tax (0.5% per transaction) or a third-party tax service
- We are merchant of record for web transactions — liable for chargebacks, refunds, VAT registration
- Ongoing maintenance burden: Apple and Stripe both evolve their APIs; we must keep both integrations current

**Cost:**

- Stripe: 1.5% + 20p per UK transaction (2.9% + 30p international)
- Stripe Tax (if used): additional 0.5% per transaction
- Apple: 15% (Small Business Program) or 30% on IAP revenue — unchanged from today
- Engineering: estimated 2-4 weeks for MVP entitlement service, ongoing maintenance

### Option B: RevenueCat + Stripe (Delegated Entitlement Layer)

RevenueCat sits between our apps and both billing systems (Stripe for web, App Store for iOS). It manages the subscription lifecycle and exposes a single entitlement API. Stripe still processes web payments, but RevenueCat orchestrates the checkout and keeps entitlement state synchronised.

**Architecture:**

```
iOS App  ──▶  RevenueCat SDK  ──▶  App Store IAP
Web App  ──▶  RevenueCat Web SDK / Stripe Checkout  ──▶  Stripe
                    │
                    ▼
            RevenueCat Backend
            (unified entitlement)
                    │
                    ▼
         Our API (query RevenueCat REST API or webhook sync to Cosmos DB)
```

**What we build:**

- RevenueCat SDK integration in iOS app (replaces direct StoreKit code)
- RevenueCat web SDK or Stripe Checkout integration on web (RevenueCat manages the Stripe connection)
- Webhook receiver for RevenueCat events (single event schema regardless of source platform)
- Optional: sync entitlement state to Cosmos DB for fast reads, or query RevenueCat's REST API directly

**What RevenueCat handles:**

- Subscription lifecycle across both Apple and Stripe
- Entitlement resolution — single source of truth for "is this user premium?"
- Receipt validation and server notification processing for both platforms
- Grace periods, billing retry, and expiry logic
- Subscriber analytics dashboard (MRR, churn, trial conversion, etc.)
- Cross-platform identity via a shared app user ID

**Pros:**

- Dramatically less entitlement code to write and maintain
- Battle-tested cross-platform subscription logic — handles the edge cases (overlapping subs, refunds, platform transfers) that are hardest to get right
- Single webhook schema instead of two different notification systems
- Built-in analytics dashboard — MRR, churn, LTV, cohort analysis out of the box
- Faster time to market — can focus engineering effort on product features instead of billing plumbing
- RevenueCat's SDKs handle StoreKit 2 migration, App Store Server Notifications v2, etc.

**Cons:**

- Additional vendor dependency — RevenueCat becomes a critical path for subscription access
- Revenue share: free up to $2,500 MTR (monthly tracked revenue), then 1% of all tracked revenue above that
- Less flexibility for exotic billing models (though standard tier-based subscriptions are well supported)
- Another identity system to integrate — RevenueCat's app user IDs must be linked to our Auth0 user IDs
- If RevenueCat has an outage, entitlement checks fail (mitigated by syncing to Cosmos DB)
- Slightly more latency if querying RevenueCat's API directly instead of a local cache

**Cost:**

- RevenueCat: free below $2,500/mo MTR, then 1% of tracked revenue
- Stripe: 1.5% + 20p per UK transaction (2.9% + 30p international) — same as Option A
- Apple: 15%/30% on IAP — unchanged
- Engineering: estimated 1-2 weeks for MVP, lower ongoing maintenance

## Options Considered

| Dimension | Option A: Stripe Only | Option B: RevenueCat + Stripe |
|-----------|----------------------|-------------------------------|
| Engineering effort (MVP) | 2-4 weeks | 1-2 weeks |
| Ongoing maintenance | High (two billing APIs) | Low (one SDK, one webhook) |
| Entitlement edge cases | We own them | RevenueCat owns them |
| Web transaction cost | ~2% | ~3% (Stripe + RC above $2.5k) |
| Vendor lock-in | Low (Stripe only) | Medium (RevenueCat) |
| Analytics | Build or buy separately | Included |
| Flexibility | Full | Standard subscription models |
| Time to revenue | Longer | Shorter |

Two other options were evaluated and ruled out:

- **Paddle** — merchant-of-record model simplifies tax but charges ~5% + 50p, has no iOS IAP unification, and offers less API flexibility than Stripe. Doesn't solve the cross-platform problem.
- **Shopify** — designed for physical goods e-commerce. Monthly platform fee, poor fit for digital subscriptions, no IAP integration.

## Recommendation

**Option B (RevenueCat + Stripe)** is the stronger choice at Town Crier's current stage.

The cross-platform entitlement problem is genuinely hard — handling the combinatorial edge cases of two independent billing systems (Apple refunds a user who also has an active Stripe subscription, user upgrades on web but iOS receipt still shows old tier, grace period semantics differ between platforms, etc.) is exactly the kind of infrastructure that consumes disproportionate engineering time relative to its user-facing value.

RevenueCat's free tier covers the early growth phase, and 1% above $2,500/mo MTR is a reasonable cost for eliminating the entitlement maintenance burden. At the point where 1% becomes material (~$250k+ MTR), the business would be well-positioned to evaluate whether to bring entitlement management in-house.

The main risk is vendor dependency. This is mitigated by: (a) syncing entitlement state to Cosmos DB so our API doesn't depend on RevenueCat availability at request time, and (b) RevenueCat's data export capabilities meaning we could migrate away without losing subscriber history.

No decision needed yet — this memo captures the analysis for further discussion.
