# Email Notification Settings — Design Spec

Date: 2026-04-06

## Goal

Wire up the email notification preferences (already stored in the domain model and accepted by PATCH /v1/me) to the web settings page, so users can configure their weekly digest and instant email notifications.

## Current State

- **Domain model** `NotificationPreferences` has `EmailDigestEnabled`, `EmailInstantEnabled`, `DigestDay` fields
- **PATCH /v1/me** accepts and persists all email fields; returns them in `UpdateUserProfileResult`
- **GET /v1/me** returns only `UserId`, `PushEnabled`, `Tier` — email fields are missing
- **Web types** `UserProfile` and `UpdateProfileRequest` have no email fields
- **Settings page** has no notifications section

## Changes

### 1. API — Expose email fields on GET /v1/me

**File: `api/src/town-crier.application/UserProfiles/GetUserProfileResult.cs`**

Add three fields to the result record:

```csharp
public sealed record GetUserProfileResult(
    string UserId,
    bool PushEnabled,
    DayOfWeek DigestDay,
    bool EmailDigestEnabled,
    bool EmailInstantEnabled,
    SubscriptionTier Tier);
```

**File: `api/src/town-crier.application/UserProfiles/GetUserProfileQueryHandler.cs`**

Map the new fields from `profile.NotificationPreferences`:

```csharp
return new GetUserProfileResult(
    profile.UserId,
    profile.NotificationPreferences.PushEnabled,
    profile.NotificationPreferences.DigestDay,
    profile.NotificationPreferences.EmailDigestEnabled,
    profile.NotificationPreferences.EmailInstantEnabled,
    profile.Tier);
```

**Tests:** Update existing GET handler tests to assert the new fields are returned. Add a test verifying default values (digest enabled, instant disabled, Monday).

### 2. Web — Update TypeScript types

**File: `web/src/domain/types.ts`**

```typescript
export type DayOfWeek =
  | 'Sunday' | 'Monday' | 'Tuesday' | 'Wednesday'
  | 'Thursday' | 'Friday' | 'Saturday';

export interface UserProfile {
  readonly userId: string;
  readonly pushEnabled: boolean;
  readonly emailDigestEnabled: boolean;
  readonly emailInstantEnabled: boolean;
  readonly digestDay: DayOfWeek;
  readonly tier: SubscriptionTier;
}

export interface UpdateProfileRequest {
  readonly pushEnabled: boolean;
  readonly emailDigestEnabled: boolean;
  readonly emailInstantEnabled: boolean;
  readonly digestDay: DayOfWeek;
}
```

### 3. Web — Add updateProfile to SettingsRepository port

**File: `web/src/domain/ports/settings-repository.ts`**

```typescript
export interface SettingsRepository {
  fetchProfile(): Promise<UserProfile>;
  updateProfile(request: UpdateProfileRequest): Promise<UserProfile>;
  exportData(): Promise<Blob>;
  deleteAccount(): Promise<void>;
}
```

**File: `web/src/features/Settings/ApiSettingsRepository.ts`**

```typescript
async updateProfile(request: UpdateProfileRequest): Promise<UserProfile> {
  return this.api.update(request);
}
```

### 4. Web — Extend useUserProfile hook

Add state and handlers for notification preferences. When a toggle or day picker changes, immediately call `repository.updateProfile()` with the full current preferences (optimistic UI — update local state first, revert on error).

Returns additional fields:
- `updateNotificationPreferences(changes: Partial<UpdateProfileRequest>): void`

The hook merges partial changes with current profile state and fires the PATCH.

### 5. Web — Notifications section in SettingsPage

Placed after the Profile section, before Appearance. Structure:

```
Notifications (h2)
┌─────────────────────────────────────┐
│ Email digest         [toggle]       │
│ ─────────────────────────────────── │
│ Digest day           [day picker]   │  ← only visible when digest enabled
│ ─────────────────────────────────── │
│ Instant emails       [toggle]       │
└─────────────────────────────────────┘
```

**Toggle component:** A new `Toggle` component in `web/src/components/Toggle/`. Renders a styled `<input type="checkbox" role="switch">` with accessible labelling. Design tokens for colours (amber accent when on, border colour when off).

**Day picker:** A native `<select>` element styled consistently with the card. Shows Monday–Sunday. Only visible when email digest is enabled.

**Field layout:** Reuses the existing `.field` class pattern (flex row, space-between, border-bottom separators).

### 6. Web — Tests

- **SettingsPage.test.tsx:** Add tests for rendering notification toggles, toggling email digest, selecting digest day, toggling instant emails, day picker visibility when digest is disabled.
- **useUserProfile.test.ts:** Test that changing preferences calls repository.updateProfile with correct payload, test optimistic update and error revert.
- **Fixtures:** Update `freeUserProfile` and `proUserProfile` fixtures to include email fields with defaults.
- **Spy:** Add `updateProfile` to `SpySettingsRepository`.

## Out of Scope

- Push notification toggle in settings UI
- Mobile (iOS) notification settings
- Any changes to the PATCH /v1/me command handler or domain model
