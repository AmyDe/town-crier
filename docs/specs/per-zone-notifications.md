# Per-Watch-Zone Notification Settings

## Context

Today notification preferences live globally on `UserProfile.NotificationPreferences` (`PushEnabled`, `EmailDigestEnabled`, `DigestDay`). The web Settings page also surfaces a dead "Instant emails" toggle wired to a deprecated Cosmos field (`UserProfileDocument.EmailInstantEnabled`) that was retired from the domain — see closed bead `tc-t94a` for the dead-toggle evidence.

The product intent is finer-grained: notification cadence and channel should be set per `WatchZone`, not globally. A user might have one zone they want push-and-email instant alerts for and another that's silent. The weekly digest is a separate concern (free-tier perk) and remains account-level.

## Tier Model

| Tier | Weekly digest (account) | Instant email (per zone) | Push (per zone) | # zones |
|---|---|---|---|---|
| Free | yes — toggle + day picker (already on web) | — | — | unchanged |
| Personal | yes | per-zone toggle | per-zone toggle | 1 |
| Pro | yes | per-zone toggle | per-zone toggle | unlimited |

Free users see no per-zone notification toggles — they are gated behind Personal/Pro. The fan-out pipeline must not deliver instant push or email to a free-tier user even if some legacy flag is set.

## Design

### Domain

`WatchZone` (currently `api/src/town-crier.domain/WatchZones/WatchZone.cs`) gains two independent booleans:

- `PushEnabled` — controls whether new applications matching this zone deliver an APNs push.
- `EmailInstantEnabled` — controls whether new applications matching this zone deliver an instant email.

Both default `true` at zone creation and on migration of existing zones (preserves current behaviour: creating a zone implies opt-in to alerts).

`WithUpdates` extends to accept the two new optional flags.

### Persistence

`WatchZoneDocument` (`api/src/town-crier.infrastructure/WatchZones/WatchZoneDocument.cs`) gains `PushEnabled` + `EmailInstantEnabled` properties with `[JsonPropertyName]` attributes. Existing Cosmos documents that lack the fields hydrate to `true` for both (Cosmos JSON deserialiser-side default — confirm AOT-safe pattern in `dotnet-coding-standards`).

No data migration job needed — null/missing → `true` on read covers it.

### API

- `WatchZoneSummary` (`api/src/town-crier.application/WatchZones/WatchZoneSummary.cs`) exposes both fields.
- `CreateWatchZoneRequest` accepts both (default `true` if omitted by older clients).
- `UpdateWatchZoneRequest` accepts both as optional.
- `WatchZoneEndpoints.PATCH` already routes to `UpdateWatchZoneCommandHandler` — wire the two new fields through.
- Source-generated `JsonSerializerContext` updated for AOT.

### Polling fan-out

The polling pipeline that emits push + email when an application matches a zone (currently flows through `DispatchNotificationEnqueuer` and the email-sending command handler — exact path documented during T1) must:

1. Skip push delivery when `WatchZone.PushEnabled == false`.
2. Skip instant email delivery when `WatchZone.EmailInstantEnabled == false`.
3. Skip both unconditionally when the user's tier is `Free`.

The weekly digest path is unaffected — it reads `UserProfile.NotificationPreferences.EmailDigestEnabled` and aggregates across all zones regardless of per-zone flags.

### iOS

- `WatchZone` model in `town-crier-domain` gains both fields.
- WatchZone editor view (Personal/Pro users only): add two `Toggle` rows ("Send push notifications", "Send instant emails"). Free users see no notification section.
- API client + repository updated for new fields on read/write.
- Settings: no change (the global "Instant emails" toggle never existed on iOS).

### Web

- Domain types in `web/src/domain` updated.
- WatchZone editor: add two toggle controls, tier-gated.
- Settings page: **remove** the dead "Instant emails" toggle in `web/src/features/Settings/SettingsPage.tsx` and the `emailInstantEnabled` field from `useUserProfile.ts`. Keep the digest toggle + day picker untouched.
- API client (`web/src/api/userProfile.ts` etc.) updated for new zone fields.

## Scope

**In:**
- Domain + persistence + API contract for `pushEnabled` / `emailInstantEnabled` per `WatchZone`.
- Polling fan-out gating both flags + tier gating (Free skipped).
- iOS WatchZone editor toggles, tier-gated.
- Web WatchZone editor toggles, tier-gated.
- Web Settings page: remove the dead "Instant emails" toggle.

**Out:**
- Master "mute all" kill-switch (do-not-disturb at OS level is sufficient).
- Per-zone weekly-digest scoping (digest stays account-level).
- Quiet hours / scheduled mute.
- Push-notification template/copy changes.

## Acceptance

- A Personal/Pro user can toggle push and email independently per zone via iOS or web; settings persist across reload and across platforms.
- The polling fan-out delivers push iff `WatchZone.PushEnabled == true` AND tier ∈ {Personal, Pro}.
- The polling fan-out delivers instant email iff `WatchZone.EmailInstantEnabled == true` AND tier ∈ {Personal, Pro}.
- Free users see no per-zone notification toggles in either client.
- Existing zones (created before this feature lands) hydrate with both flags `true`; no behaviour change for current users.
- The dead "Instant emails" toggle is gone from web Settings.

## Steps

### T1 — API: domain + persistence + endpoints

`WatchZone` entity + `WatchZoneDocument` + `CreateWatchZoneCommand` / `UpdateWatchZoneCommand` and request/result DTOs all carry `PushEnabled` + `EmailInstantEnabled`. AOT-safe JSON serialisation. PATCH `/v1/me/watch-zones/{id}` accepts both fields. Existing Cosmos docs without the fields hydrate to `true`. Handler tests for create + update cover the new flags.

### T2 — API: polling fan-out gating

Wire the two flags + tier check into the push and instant-email delivery paths. Add handler tests proving that:
- Push is suppressed when `PushEnabled == false`.
- Instant email is suppressed when `EmailInstantEnabled == false`.
- Both are suppressed when tier is Free, regardless of flag values.
- Weekly digest path is unaffected.

Depends on T1 (uses the new fields).

### T3 — iOS: WatchZone editor toggles

Domain model + repository + editor view gain the two toggles, tier-gated (hidden for Free). Wire create + update through the API client. ViewModel tests for the toggle behaviour and tier gating.

Depends on T1 (consumes new API contract).

### T4 — Web: WatchZone editor toggles

Domain types + zone editor + API client gain the toggles, tier-gated. Vitest coverage for toggle behaviour and tier gating.

Depends on T1 (consumes new API contract).

### T5 — Web: remove dead "Instant emails" Settings toggle

Delete the toggle from `SettingsPage.tsx` and the `emailInstantEnabled` field from `useUserProfile.ts` and the API client. Keep digest toggle + day picker unchanged. Vitest update for the Settings tests.

Independent of T1–T4 (deletion-only on web Settings, no API contract dependency).
