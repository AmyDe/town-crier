# Email Notifications

## Overview

Two email features sharing a single `IEmailSender` port backed by Azure Communication Services (ACS):

1. **Weekly digest email** — available to all tiers. A daily Container Apps Job runs `GenerateWeeklyDigestsCommand`, filters users whose `DigestDay` matches the current day, builds a grouped card-style HTML email per user, and sends via ACS.
2. **Instant notification email** — available to Personal/Pro tiers only. `DispatchNotificationCommandHandler` (change feed path) sends a per-application email alongside the existing push notification. No monthly cap for paid tiers.

Sender address for all emails: `hello@towncrierapp.uk`.

## Port & Adapter

New application port mirroring the existing `IPushNotificationSender` pattern:

```csharp
public interface IEmailSender
{
    Task SendDigestAsync(string email, IReadOnlyList<WatchZoneDigest> digests, CancellationToken ct);
    Task SendNotificationAsync(string email, Notification notification, CancellationToken ct);
}
```

`WatchZoneDigest` is a new DTO grouping notifications by watch zone:

```csharp
public sealed record WatchZoneDigest(string WatchZoneName, IReadOnlyList<Notification> Notifications);
```

Infrastructure implementations:
- `AcsEmailSender` — production adapter using the Azure Communication Services Email SDK.
- `NoOpEmailSender` — dev/test adapter that does nothing.

## User Preferences

Extend `NotificationPreferences` with two new fields:

| Field | Type | Default | Tier restriction |
|-------|------|---------|-----------------|
| `EmailDigestEnabled` | bool | `true` | None (all tiers) |
| `EmailInstantEnabled` | bool | `false` | Only honoured for Personal/Pro |

The user's email address is already stored on the user profile document in Cosmos (set during onboarding). No Auth0 lookup is needed at send time.

## Weekly Digest Email

### Trigger

A daily Container Apps Job runs on a cron schedule (e.g. `0 8 * * *` — 08:00 UTC daily). It invokes the existing `GenerateWeeklyDigestsCommand`.

### Handler Changes

The current `GenerateWeeklyDigestsCommandHandler`:
1. Loads all Pro users.
2. Filters by `DigestDay == today`, `PushEnabled`, and notification count > 0 in the past 7 days.
3. Sends a push digest with the application count.

Changes required:
- **Load all users** (not just Pro) who have `EmailDigestEnabled == true` and `DigestDay == today`.
- **Load the actual notification records** for the past 7 days (not just the count). This requires a new repository method or reuse of an existing one that returns `IReadOnlyList<Notification>`.
- **Load watch zones** for each user to group notifications by zone.
- **Build `WatchZoneDigest` list** by matching notifications to watch zones.
- **Call `IEmailSender.SendDigestAsync()`** with the user's email and the grouped digests.
- **Preserve existing push digest behaviour** — Pro users with `PushEnabled` still get push digests as before.

### Repository Extension

`INotificationRepository` needs a method to return full notification records for a user since a given date:

```csharp
Task<IReadOnlyList<Notification>> GetByUserSinceAsync(string userId, DateTimeOffset since, CancellationToken ct);
```

Note: `CountByUserSinceAsync` already exists — this is the list-returning counterpart.

### Email Content

Card-per-application layout grouped by watch zone:

```
┌──────────────────────────────────┐
│         Town Crier               │
│     Weekly Planning Digest       │
├──────────────────────────────────┤
│ 📍 Home — SE1 Watch Zone        │
│ ┌──────────────────────────────┐ │
│ │ 14 Elm Street               │ │
│ │ Householder Application     │ │
│ │ Single storey rear ext...   │ │
│ │ View details →              │ │
│ └──────────────────────────────┘ │
│ ┌──────────────────────────────┐ │
│ │ 3 Oak Lane                  │ │
│ │ Listed Building Consent     │ │
│ │ Replacement of timber wi... │ │
│ │ View details →              │ │
│ └──────────────────────────────┘ │
│                                  │
│ 📍 Office — EC2 Watch Zone      │
│ ┌──────────────────────────────┐ │
│ │ 100 Bishopsgate             │ │
│ │ Change of Use               │ │
│ │ Conversion of ground fl...  │ │
│ │ View details →              │ │
│ └──────────────────────────────┘ │
│                                  │
│        [ View All in App ]       │
│                                  │
│  5 applications · Unsubscribe    │
└──────────────────────────────────┘
```

HTML is built inline in `AcsEmailSender` — no templating engine. Keep it simple, table-based HTML for email client compatibility.

## Instant Notification Email

### Trigger

Fires from the existing `DispatchNotificationCommandHandler` (change feed processor path) when a new planning application matches a user's watch zone.

### Handler Changes

After the existing push notification logic:
- Check if the user is Personal/Pro tier **and** has `EmailInstantEnabled == true`.
- If so, call `IEmailSender.SendNotificationAsync()` with the user's email and the notification.
- No monthly cap for paid-tier email notifications.

### Email Content

Mirrors push notification content:
- Application address
- Application type
- Application description
- Link to view in app

Same inline HTML approach as the digest, but a simpler single-application layout.

## Infrastructure (Pulumi)

### Shared Stack

- **Azure Communication Services** resource.
- **Email Communication Service** linked to the ACS resource.
- **Custom domain** `towncrierapp.uk` on the Email Communication Service.
- **Sender address** `hello@towncrierapp.uk`.

### DNS (Cloudflare)

Verification records for the custom domain:
- SPF record
- DKIM records (provided by ACS during domain verification)
- DMARC record

### Environment Stack

- ACS connection string passed to the Container App as an environment variable or secret.
- Daily Container Apps Job for the digest trigger (cron schedule, shares the same container image as the API).

## Error Handling

- Failed email sends should be logged but not block the handler (same pattern as failed push notifications).
- ACS SDK throws on transient failures — wrap calls with retry policy or let the Container Apps Job retry on failure.
- If a user has no email address on their profile, skip email delivery silently (log a warning).

## Testing

- `SpyEmailSender` test double (mirrors `SpyPushNotificationSender`) — records sent emails for assertions.
- Unit tests for `GenerateWeeklyDigestsCommandHandler` covering:
  - Free user with `EmailDigestEnabled` receives digest email.
  - User with `EmailDigestEnabled == false` does not receive email.
  - Pro user receives both push and email digest.
  - Notifications grouped correctly by watch zone.
  - User with no email address is skipped.
- Unit tests for `DispatchNotificationCommandHandler` covering:
  - Personal/Pro user with `EmailInstantEnabled` receives instant email.
  - Free user does not receive instant email regardless of setting.
  - Email mirrors notification content.
