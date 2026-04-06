# Entitlement Framework

Date: 2026-04-06

## Problem

Tier-gated features are enforced ad-hoc: some handlers check `profile.Tier` manually, some don't check at all. The notification preferences update endpoint accepts `emailInstantEnabled = true` from free-tier users without resistance. A user with Postman (or curiosity) can grant themselves paid features.

As the feature set grows, every new gated endpoint repeats the same pattern — fetch profile, check tier, return 403. This is error-prone and inconsistent.

## Design

Three enforcement points share a single authority (`EntitlementMap`) for resolving what each tier grants.

### Entitlement Model (Domain Layer)

An `Entitlement` enum defines every gatable feature:

```csharp
public enum Entitlement
{
    InstantEmails,
    SearchApplications,
    StatusChangeAlerts,
    DecisionUpdateAlerts
}
```

A static `EntitlementMap` resolves tier to entitlements:

```csharp
public static class EntitlementMap
{
    public static IReadOnlySet<Entitlement> For(SubscriptionTier tier) => tier switch
    {
        SubscriptionTier.Personal => new HashSet<Entitlement>
        {
            Entitlement.InstantEmails,
            Entitlement.StatusChangeAlerts,
            Entitlement.DecisionUpdateAlerts
        },
        SubscriptionTier.Pro => new HashSet<Entitlement>
        {
            Entitlement.InstantEmails,
            Entitlement.SearchApplications,
            Entitlement.StatusChangeAlerts,
            Entitlement.DecisionUpdateAlerts
        },
        _ => new HashSet<Entitlement>()
    };
}
```

New entitlements: add an enum value and update the mapping. Endpoints and Auth0 Action don't change.

### Endpoint Filter (Web Layer)

A `RequiresEntitlementAttribute` marks endpoints declaratively. An `EntitlementEndpointFilter` runs before the handler:

1. Reads `subscription_tier` from `ClaimsPrincipal` (defaults to `Free` if missing — fail-safe for tokens issued before the Action was deployed)
2. Parses to `SubscriptionTier` enum
3. Calls `EntitlementMap.For(tier)` to get the entitlement set
4. If the required entitlement is absent: returns 403 with structured error body
5. Otherwise: continues to handler

Usage on an endpoint:

```csharp
group.MapPatch("/me/preferences", Handler)
    .WithMetadata(new RequiresEntitlementAttribute(Entitlement.InstantEmails));
```

403 response body:

```json
{
    "error": "insufficient_entitlement",
    "required": "InstantEmails",
    "message": "This feature requires a paid subscription."
}
```

### Background Job Enforcement

Background jobs (notification dispatch, digest generation) have no HTTP request or JWT. They read `UserProfile.Tier` from Cosmos and call the same `EntitlementMap.For(tier)`:

```csharp
var entitlements = EntitlementMap.For(profile.Tier);

if (entitlements.Contains(Entitlement.InstantEmails)
    && profile.NotificationPreferences.EmailInstantEnabled
    && !string.IsNullOrEmpty(profile.Email))
{
    await this.emailSender.SendNotificationAsync(...);
}
```

One mapping, two code paths, consistent behaviour.

### Auth0 Post-Login Action

A Post-Login Action adds `subscription_tier` to every access token from `app_metadata`:

```js
exports.onExecutePostLogin = async (event, api) => {
  const tier = event.user.app_metadata?.subscription_tier || 'Free';
  api.accessToken.setCustomClaim('subscription_tier', tier);
};
```

No external calls in the Action — reads metadata already on the user object.

### Auth0 Metadata Sync

When a user's tier changes (admin grant, App Store webhook, future Stripe webhook), the handler updates both Cosmos and Auth0:

1. Update `UserProfile.Tier` in Cosmos (source of truth)
2. PATCH Auth0 Management API: `app_metadata.subscription_tier`

This uses an M2M application with `update:users` scope (available on Auth0 free tier).

### Client Token Refresh

After purchase, clients force a token refresh so the new claim is picked up immediately:

- iOS: `credentialsManager.renew()`
- Web: `getAccessTokenSilently({ cacheMode: 'off' })`

Between purchases, the claim refreshes naturally when the access token expires.

## Enforcement Summary

| Context | Mechanism | Tier source |
|---------|-----------|-------------|
| HTTP endpoints | `RequiresEntitlementAttribute` + `EntitlementEndpointFilter` | JWT `subscription_tier` claim |
| Background jobs | `EntitlementMap.For(profile.Tier)` | `UserProfile.Tier` from Cosmos |
| Both | `EntitlementMap` resolves tier to entitlements | — |

## Key Decisions

- **Named entitlements, not tier levels** on endpoints — decouples features from pricing tiers. Rearranging what each tier includes means changing the mapping, not every endpoint.
- **JWT claim defaults to `Free` if missing** — fail-safe, never accidentally grants access.
- **Stale preferences on downgrade are acceptable** — if a user downgrades with `emailInstantEnabled = true`, the preference persists in Cosmos but is never acted on. Re-subscribing restores their settings.
- **Existing ad-hoc tier checks** (`ProTierRequiredException` in search, `InsufficientTierException` in zone preferences) migrated to the new pattern over time, not urgently.

## Migration Path

1. Add `Entitlement` enum and `EntitlementMap` to domain layer
2. Add `RequiresEntitlementAttribute` and `EntitlementEndpointFilter` to web layer
3. Deploy Auth0 Post-Login Action
4. Add Auth0 metadata sync to `GrantSubscriptionCommandHandler`
5. Apply `RequiresEntitlement` to the notification preferences endpoint (fixes the original bug)
6. Update background jobs to use `EntitlementMap`
7. Gradually migrate existing ad-hoc checks to the attribute pattern
