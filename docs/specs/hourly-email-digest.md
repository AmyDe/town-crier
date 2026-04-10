# Hourly Email Digest

## Context

Instant email notifications fire one email per planning application per user. In busy areas this causes email floods (100+ emails overnight), quickly exhausting the ACS free-tier quota of 100 emails/day. Planning applications move on timescales of weeks — sub-hour email latency has no practical value.

This work replaces instant per-application emails with an hourly batched digest, and adds safeguards to prevent historical applications from triggering notifications on new WatchZones.

See: ADR-0009 (notification architecture), ADR-0020 (ACS email), `docs/specs/email-notifications.md`.

## Design

### 1. Reduce first-poll lookback to 1 day

`PollPlanItCommandHandler` currently defaults to a 30-day lookback when no poll state exists for an authority (line 65). Change this to 1 calendar day. The 30-day window served initial development; in production, once an authority is being polled, the high water mark provides continuous coverage. A 1-day lookback limits the first-poll burst to a handful of applications.

**File:** `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs`
**Change:** `now.AddDays(-30)` → `now.AddDays(-1)`

### 2. Add `CreatedAt` to WatchZone

The `WatchZone` domain entity has no creation timestamp. Without it, there's no way to filter out applications that predate the zone. When a user creates a zone at 3pm but the authority was last polled at 8am, the next poll surfaces applications filed before the user existed — making the system feel broken.

**Domain change:** Add `DateTimeOffset CreatedAt` property to `WatchZone`, set in the constructor and `Create` factory. Add to `Reconstitute` for hydration from Cosmos.

**Polling filter:** In `PollPlanItCommandHandler`, after finding matching zones for an application, skip zones where `zone.CreatedAt > application.LastDifferent`. This ensures only applications that changed *after* the zone was created generate notifications.

**Cosmos migration:** Existing WatchZone documents have no `createdAt` field. The `Reconstitute` method should default missing values to `DateTimeOffset.MinValue` (effectively: all existing zones see all applications, preserving current behaviour for existing users).

**Files:**
- `api/src/town-crier.domain/WatchZones/WatchZone.cs`
- `api/src/town-crier.infrastructure/WatchZones/CosmosWatchZoneRepository.cs` (document mapping)
- `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs` (filter)
- `api/src/town-crier.application/WatchZones/CreateWatchZoneCommandHandler.cs` (pass `now`)

### 3. Add `EmailSent` flag to Notification

The `Notification` entity tracks `PushSent` but has no equivalent for email. The hourly digest job needs to know which notifications haven't been emailed yet.

**Domain change:** Add `bool EmailSent` property to `Notification` with a `MarkEmailSent()` method, mirroring `MarkPushSent()`.

**Repository change:** Add `GetUnsentEmailsByUserAsync(string userId, CancellationToken ct)` to `INotificationRepository` — returns notifications where `EmailSent == false`, ordered by `CreatedAt`.

**Files:**
- `api/src/town-crier.domain/Notifications/Notification.cs`
- `api/src/town-crier.application/Notifications/INotificationRepository.cs`
- `api/src/town-crier.infrastructure/Notifications/CosmosNotificationRepository.cs`

### 4. Remove instant email from DispatchNotificationCommandHandler

The dispatch handler currently sends an instant email for Pro users with `EmailInstantEnabled` (lines 117-125). Remove this block entirely. Email is now handled exclusively by the hourly digest job.

The `EmailInstantEnabled` user preference becomes unused. Remove it from `NotificationPreferences` and mark the Cosmos field as ignored (existing documents may still have it).

**Files:**
- `api/src/town-crier.application/Notifications/DispatchNotificationCommandHandler.cs` (remove email block)
- `api/src/town-crier.domain/UserProfiles/NotificationPreferences.cs` (remove `EmailInstantEnabled`)

### 5. Hourly digest handler

New `GenerateHourlyDigestsCommandHandler` — modelled on the existing weekly digest handler but runs for all users with pending unsent-email notifications.

**Flow:**
1. Query all distinct user IDs that have notifications with `EmailSent == false`
2. For each user, load profile — skip if no email or `EmailDigestEnabled == false`
3. Load unsent notifications via `GetUnsentEmailsByUserAsync`
4. Group by WatchZone, build `WatchZoneDigest` list (reuse existing DTO)
5. Send via `IEmailSender.SendDigestAsync` (reuse existing email template)
6. Mark all included notifications as `EmailSent = true` and save

**Entitlement check:** Only send hourly emails to users whose tier includes the `HourlyDigestEmails` entitlement (Personal/Pro). Free-tier users still get the weekly digest only.

**Empty batches:** If a user has no unsent notifications, skip them. If no users have unsent notifications, the job completes immediately with no ACS calls.

**Repository additions:**
- `GetUserIdsWithUnsentEmailsAsync(CancellationToken ct)` on `INotificationRepository`
- `GetUnsentEmailsByUserAsync(string userId, CancellationToken ct)` on `INotificationRepository`

**Files:**
- New: `api/src/town-crier.application/Notifications/GenerateHourlyDigestsCommandHandler.cs`
- New: `api/src/town-crier.application/Notifications/GenerateHourlyDigestsCommand.cs`
- `api/src/town-crier.application/Notifications/INotificationRepository.cs`
- `api/src/town-crier.infrastructure/Notifications/CosmosNotificationRepository.cs`

### 6. Worker mode + Container Apps Job

Add `"hourly-digest"` mode to the worker `Program.cs`, following the existing `"digest"` pattern. Wire up `GenerateHourlyDigestsCommandHandler` in the DI container.

Add a Pulumi Container Apps Job with cron schedule `0 * * * *` (top of every hour).

**Files:**
- `api/src/town-crier.worker/Program.cs`
- `infra/` (Pulumi job definition)

### 7. Update entitlements

Add `HourlyDigestEmails` to the `Entitlement` enum and wire it into `EntitlementMap` for Personal and Pro tiers. The existing `InstantEmails` entitlement becomes unused — remove it.

**Files:**
- `api/src/town-crier.domain/Entitlements/Entitlement.cs`
- `api/src/town-crier.domain/Entitlements/EntitlementMap.cs`

## Scope

**In scope:**
- All seven changes above
- Tests for each change (TDD per coding standards)
- ADR update: amend ADR-0020 to reflect hourly digest replacing instant emails
- Update `docs/specs/email-notifications.md` to reflect new architecture

**Out of scope:**
- Rethinking the weekly digest (may become redundant but that's a separate decision)
- User-configurable email cadence (instant vs hourly vs daily) — hourly is the only option
- iOS or web UI changes for email preferences (removing `EmailInstantEnabled` toggle, if one exists)
