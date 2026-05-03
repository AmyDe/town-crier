# Notification Preferences: Authorization State Awareness

GH: https://github.com/AmyDe/town-crier/issues/360

## Status

Open

## Problem

When the user has never granted (or revoked) notification permission, iOS has no per-app Notifications subpage. The deep-link to `openNotificationSettingsURLString` falls through to the system-wide Notifications hub instead. The Notification Preferences screen should detect `UNAuthorizationStatus` and render accordingly.

## States

| Status | Section shown | Footer link |
|---|---|---|
| `.notDetermined` | "Turn on notifications" button | Hidden (subpage doesn't exist yet) |
| `.denied` | Explanatory copy, "turn on in iOS Settings" | Visible |
| `.authorized` / `.provisional` / `.ephemeral` | None | Visible (current behaviour) |
| `nil` (still loading) | None (no flash of wrong state) | Visible |

Refresh on `scenePhase == .active` so changes made in iOS Settings are reflected without a manual reload.

---

## Phase 1 — Domain enum + protocol + infrastructure adapter (foundational)

**Bead: tc-notifperms-plumbing**

### New domain type

`mobile/ios/packages/town-crier-domain/Sources/ValueObjects/NotificationAuthorizationStatus.swift` (new):
```swift
public enum NotificationAuthorizationStatus: Sendable, Equatable {
  case notDetermined
  case denied
  case authorized  // covers UNAuthorizationStatus.{authorized, provisional, ephemeral}
}
```

### Protocol changes

`mobile/ios/packages/town-crier-domain/Sources/Protocols/NotificationPermissionProvider.swift`:
```swift
func authorizationStatus() async -> NotificationAuthorizationStatus
```

`mobile/ios/packages/town-crier-domain/Sources/Protocols/NotificationService.swift`:
- Add the same `authorizationStatus() async -> NotificationAuthorizationStatus`

### Infrastructure adapter

`mobile/ios/town-crier-app/Sources/UNNotificationPermissionProvider.swift`:
```swift
func authorizationStatus() async -> NotificationAuthorizationStatus {
  let settings = await UNUserNotificationCenter.current().notificationSettings()
  switch settings.authorizationStatus {
  case .notDetermined: return .notDetermined
  case .denied: return .denied
  case .authorized, .provisional, .ephemeral: return .authorized
  @unknown default: return .denied  // fail closed
  }
}
```

`mobile/ios/packages/town-crier-data/Sources/Notifications/CompositeNotificationService.swift`:
- Add passthrough to `permissionProvider.authorizationStatus()`

### Tests

- `SpyNotificationPermissionProvider.swift`: add `var nextAuthorizationStatus: NotificationAuthorizationStatus = .authorized` and `authorizationStatusCallCount: Int`.
- `CompositeNotificationServiceTests.swift`: cover the new passthrough.
- `CompositionRootTests.swift`: update if needed (new method on spy).

**Files (Phase 1):**
- `mobile/ios/packages/town-crier-domain/Sources/ValueObjects/NotificationAuthorizationStatus.swift` (new)
- `mobile/ios/packages/town-crier-domain/Sources/Protocols/NotificationPermissionProvider.swift`
- `mobile/ios/packages/town-crier-domain/Sources/Protocols/NotificationService.swift`
- `mobile/ios/town-crier-app/Sources/UNNotificationPermissionProvider.swift`
- `mobile/ios/packages/town-crier-data/Sources/Notifications/CompositeNotificationService.swift`
- `mobile/ios/town-crier-tests/Sources/Spies/SpyNotificationPermissionProvider.swift`
- `mobile/ios/town-crier-tests/Sources/Features/CompositeNotificationServiceTests.swift`
- `mobile/ios/town-crier-tests/Sources/Features/CompositionRootTests.swift`

---

## Phase 2 — ViewModel + View permission section

**Bead: tc-notifperms-ui** (depends on Phase 1)

### ViewModel

`NotificationPreferencesViewModel.swift`:
- Add `@Published public private(set) var authorizationStatus: NotificationAuthorizationStatus? = nil`.
- Inject `notificationService: NotificationService` in init (coordinator already has it at `AppCoordinator.notificationService:41`).
- In `load()`: query `await notificationService.authorizationStatus()` in parallel with profile/zone loads.
- Add `public func requestPermission() async`:
  - call `notificationService.requestPermission()` (error → `handleError`)
  - re-query and assign `authorizationStatus`
- Add `public func refreshAuthorizationStatus() async` for foreground-refresh.

`AppCoordinator.makeNotificationPreferencesViewModel()`: pass `notificationService`.

### View

`NotificationPreferencesView.swift`:
- `permissionSection` rendered above "Saved Applications":
  - `nil` → nothing (no loading flash)
  - `.notDetermined` → Section header "Notifications", body "Town Crier needs permission to send you notifications.", `Button("Turn on notifications")` → `await viewModel.requestPermission()`
  - `.denied` → Section header "Notifications", body "Notifications are turned off for Town Crier. Turn them on in iOS Settings to receive alerts."
  - `.authorized` → no section
- Footer link ("Open iOS notification settings") **hidden** when `authorizationStatus == .notDetermined`.
- `.onChange(of: scenePhase)`: when transitioning to `.active`, call `await viewModel.refreshAuthorizationStatus()`.

### Tests

`NotificationPreferencesViewModelTests.swift` — new tests:
- `loadPopulatesAuthorizationStatus_notDetermined` / `_denied` / `_authorized`
- `requestPermissionRefreshesAuthorizationStatus`
- `requestPermissionFailureSurfacesError` (error → `handleError`, status still re-queried)

`NotificationPreferencesViewTests.swift` — construction + permission-button callback test.

**Files (Phase 2):**
- `mobile/ios/packages/town-crier-presentation/Sources/Features/NotificationPreferences/NotificationPreferencesViewModel.swift`
- `mobile/ios/packages/town-crier-presentation/Sources/Features/NotificationPreferences/NotificationPreferencesView.swift`
- `mobile/ios/packages/town-crier-presentation/Sources/Coordinators/AppCoordinator.swift`
- `mobile/ios/town-crier-tests/Sources/Features/NotificationPreferencesViewModelTests.swift`
- `mobile/ios/town-crier-tests/Sources/Features/NotificationPreferencesViewTests.swift`

---

## Out of Scope

- Modifying onboarding permission flow
- Distinguishing `.provisional` / `.ephemeral` from `.authorized` in UI
- Non-iOS platforms (domain enum compiles clean via `#if canImport(UIKit)` pattern)
- Deep-link URL change (already done in PR #359)

## Related

- tc-bzjx epic: in-app notification preferences screen (#355)
- tc-kdik: deep-link to iOS Notifications subpage (#357, PR #359)
