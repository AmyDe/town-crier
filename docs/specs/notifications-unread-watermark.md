# Notifications Unread Watermark + Applications Screen Read-State

GH: https://github.com/AmyDe/town-crier/issues/365

## Overview

Add server-side watermark-based read-tracking for notifications. Surface it on the Applications screen across iOS and web as an `Unread` filter chip, per-row status pill, Mark-All-Read button, iOS badge, and sort menu. Decommission the standalone web `/notifications` page in the same release.

## Pre-Resolved Decisions

1. Read state lives server-side (`notification-state/{userId}` Cosmos doc with `lastReadAt`).
2. Watermark model (single `lastReadAt` timestamp), not per-row `ReadAt`.
3. Manual mark-all-read only — no auto-advance on tab visit.
4. Pivot to Applications screen; decommission web `/notifications`.
5. 5th iOS tab stays empty — not in scope.
6. Event-typed status pill on rows, persisting muted after read.
7. `Unread` is one more value in the existing single-select Status chip group.
8. Mark-All-Read: conditional visibility, trailing, silent optimistic.
9. Default sort: `max(receivedDate, latestUnreadEvent.createdAt)` desc; deferred resort.
10. Sort: 4 client-side options persisted under `applicationsListSort`.
11. Push-tap advances watermark to tapped notification's `createdAt`; monotonic server-side.
12. Augment Applications endpoint with `latestUnreadEvent`; thin sibling endpoint for badge/watermark.
13. Migration: clean slate (`lastReadAt = deploy_time_utc` for all existing users).
14. No silent push for cross-device badge sync; pull-on-foreground instead.
15. Existing `Notification` aggregate unchanged.

## Data Model

New Cosmos document `notification-state/{userId}`:
```
{ id: userId, partitionKey: userId, lastReadAt: DateTimeOffset, version: int }
```

A notification is "unread" iff `notification.createdAt > userNotificationState.lastReadAt`.

## API Surface

1. **Augment** `GET /v1/me/watch-zones/{zoneId}/applications` — each row gains `latestUnreadEvent: { type, decision, createdAt } | null`
2. **New** `GET /v1/me/notification-state` → `{ lastReadAt, totalUnreadCount }`
3. **New** `POST /v1/me/notification-state/mark-all-read` → 204
4. **New** `POST /v1/me/notification-state/advance` body `{ asOf }` → 204 (monotonic)
5. **Remove** `GET /v1/notifications`, `NotificationEndpoints.cs`, and all related query handlers/types
6. **Fix** `ApnsPushNotificationSender.cs` — replace hardcoded `Badge: 1` with actual `totalUnreadCount`

## Tasks

### #api-domain (P1)
New `NotificationState` aggregate root in `api/src/town-crier.domain/NotificationState/` with factory, `MarkAllReadAt(now)`, `AdvanceTo(asOf)`. Port `INotificationStateRepository`. Cosmos impl in new container `notification-state` (`/userId` partition).

### #api-endpoints (P2, depends on #api-domain)
`NotificationStateEndpoints.cs` for GET state, POST mark-all-read, POST advance. Register in `WebApplicationExtensions.cs`. Add DTOs to `AppJsonSerializerContext`.

### #api-augment-applications (P2, depends on #api-domain)
Augment `GetApplicationsByZoneQueryHandler` + result DTO with `latestUnreadEvent`. Add `GetLatestUnreadEventByApplicationAsync` + `GetUnreadCountAsync` to `INotificationRepository` and implement in `CosmosNotificationRepository`.

### #api-badge-fix (P2, depends on #api-domain)
Fix `ApnsPushNotificationSender.cs:89` hardcoded `Badge: 1`. Inject state/notification repos to compute `totalUnreadCount` post-record. Apply to both alert and digest payloads.

### #api-decom-notifications (P2)
Delete `NotificationEndpoints.cs`, `GetNotificationsQueryHandler.cs`, `GetNotificationsQuery.cs`, `GetNotificationsResult.cs`, `NotificationItem.cs`. Remove `MapNotificationEndpoints` call.

### #ios-delete-dead-notifications (P2)
Delete `mobile/ios/packages/town-crier-presentation/Sources/Features/Notifications/` folder (3 files). Delete any `Tests/.../Features/Notifications/` tests. Lift colour palette/SF Symbols from `NotificationDecisionBadge.swift` into new `ApplicationStatusPill.swift` before deleting.

### #ios-notification-state-domain (P2, depends on #api-endpoints)
New `NotificationState.swift` value object + `NotificationStateRepository` protocol in domain package. `ApiNotificationStateRepository.swift` in data package hitting the 3 new endpoints.

### #ios-applications-unread-ui (P2, depends on #ios-notification-state-domain, #ios-delete-dead-notifications)
`ApplicationListView`: add Unread chip (with count), sort `ToolbarItem` (4 options, persisted in UserDefaults), Mark-All-Read `ToolbarItem` (conditional). `ApplicationListRow`: render `ApplicationStatusPill` with saturation driven by `latestUnreadEvent`.

### #ios-badge-foreground-push (P2, depends on #ios-notification-state-domain)
AppDelegate/scene foreground hook → fetch notification-state → reconcile `applicationIconBadgeNumber`. `AppCoordinator+DeepLink.swift`: fire `advance` with tapped notification's `createdAt` on push-tap. Extend `NotificationPayloadParser.swift` for `createdAt`.

### #web-decom-notifications (P2)
Delete `web/src/features/Notifications/`, `web/src/api/notifications.ts`, `web/src/domain/ports/notification-repository.ts`. Remove `/notifications` route (AppRoutes.tsx:73). Remove dashboard quick link (DashboardPage.tsx:41). Update `DashboardPage.test.tsx:61`. Audit `SettingsPage.test.tsx:185`.

### #web-applications-unread-ui (P2, depends on #api-endpoints)
`ApplicationsPage.tsx`: Unread chip, sort button, Mark-All-Read button. `useApplications.ts`: unread state, sort, mark-all-read, advance. `ApplicationCard.tsx`: read-state-aware status pill. New `web/src/api/notification-state.ts` + port + implementation.
