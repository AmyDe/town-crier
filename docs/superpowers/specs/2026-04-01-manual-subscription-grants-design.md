# Manual Subscription Grants Design

Date: 2026-04-01

## Problem

The web app needs feature parity with the iOS app, including tier-gated features (watch zone limits, radius options, application filtering). But there's no payment mechanism for the web app yet, and the App Store subscription flow isn't fully wired up on the API side either.

During the build phase, we need the ability to manually grant subscription tiers to accounts for:
- Testing tier-gated features end-to-end
- Giving friends and family early access at any tier

## Decision

Add an admin API endpoint that sets a user's subscription tier by email address. Secure it with a shared API key. Store email on the UserProfile document in Cosmos so the lookup is self-contained (no Auth0 Management API dependency).

Manual grants use a far-future expiry (2099-12-31) so they're effectively permanent until explicitly revoked.

## Admin Endpoint

**Route:** `PUT /v1/admin/subscriptions`

**Authentication:** Shared API key via `X-Admin-Key` request header. The key is stored in app configuration (`AdminApiKey` or equivalent environment variable). Requests with a missing or incorrect key receive `401 Unauthorized`.

**Request body:**
```json
{
  "email": "friend@example.com",
  "tier": "Personal"
}
```

- `email` (required): The email address of the target user. Must match an existing UserProfile.
- `tier` (required): One of `Free`, `Personal`, or `Pro`. Setting `Free` revokes a previous grant.

**Response:**
- `200 OK` with the updated user profile on success
- `401 Unauthorized` if the API key is missing or invalid
- `404 Not Found` if no UserProfile exists with that email
- `400 Bad Request` if tier value is invalid

**Behaviour:**
1. Validate the `X-Admin-Key` header against the configured secret
2. Query the Cosmos `UserProfiles` container by `email` field
3. If tier is `Free`: call `ExpireSubscription()` on the domain model
4. If tier is `Personal` or `Pro`: call `ActivateSubscription(tier, 2099-12-31)`
5. Persist the updated document to Cosmos
6. Return the updated profile

**Usage (curl):**
```bash
curl -X PUT https://api.dev.towncrierapp.uk/v1/admin/subscriptions \
  -H "X-Admin-Key: your-secret-key" \
  -H "Content-Type: application/json" \
  -d '{"email": "friend@example.com", "tier": "Pro"}'
```

## Email on UserProfile

The UserProfile document in Cosmos currently has no `email` field. The Auth0 token used at profile creation contains the user's email in its claims.

**Changes:**
- Add `email` string field to `UserProfileDocument`
- Populate it from the Auth0 `email` claim when creating a profile (`POST /v1/me`)
- The email field is stored but not used for authentication (Auth0 user ID remains the identity key)

**Cosmos query:** The admin endpoint queries UserProfiles by email. This is a cross-partition query since profiles are partitioned by user ID. For an admin tool used infrequently, this is acceptable without adding a secondary index.

## Scope

**In scope:**
- Admin endpoint for setting subscription tier by email
- API key authentication middleware/filter for admin routes
- Adding email to UserProfileDocument at profile creation
- CQRS command + handler for the grant operation

**Out of scope:**
- Web admin UI (curl is sufficient for now)
- Batch grants (one user at a time)
- Audit logging of grants (the Cosmos document itself is the record)
- Backfilling email on existing UserProfile documents (handle manually if needed)
- Web app feature gating (separate work — the web app already has a `ProGate` component and fetches the tier from `GET /v1/me`)

## Architecture

Follows existing patterns:
- **Command:** `GrantSubscriptionCommand` with `Email` and `Tier` fields
- **Handler:** `GrantSubscriptionCommandHandler` — validates key, queries by email, mutates domain model, persists
- **Endpoint:** Minimal API route in the admin area, manually dispatching to the handler
- **Auth middleware:** A simple middleware or endpoint filter that checks `X-Admin-Key` against configuration. Scoped to `/v1/admin/*` routes only.

No new NuGet packages required. No reflection. Native AOT compatible.
