# In-App Notification Preferences Screen

GH: https://github.com/AmyDe/town-crier/issues/355

## Status

Open

## Problem

Tapping "Notification Preferences" in Settings deep-links straight to iOS system Settings (delivery controls only). The app's own preference toggles (`savedDecisionPush`, `savedDecisionEmail`, `emailDigestEnabled`, `digestDay`) are scattered or not surfaced at all — email digest toggles exist in the API but appear nowhere in the UI.

## Goal

Replace the deep-link row with an in-app `NotificationPreferencesView` that owns *what* triggers a notification. System Settings is still reachable from a footer link on the new screen for *how* notifications are delivered.

## No API changes needed

`PATCH /v1/me` already accepts all five preference fields (`savedDecisionPush`, `savedDecisionEmail`, `pushEnabled`, `emailDigestEnabled`, `digestDay`). No backend changes required.

---

## Phase 1 — ViewModel (foundational)

**Bead: tc-notifprefs-vm**

New file: `mobile/ios/packages/town-crier-presentation/Sources/Features/NotificationPreferences/NotificationPreferencesViewModel.swift`

```swift
@MainActor
public final class NotificationPreferencesViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public private(set) var savedDecisionPush: Bool = true
  @Published public private(set) var savedDecisionEmail: Bool = true
  @Published public private(set) var emailDigestEnabled: Bool = true
  @Published public private(set) var digestDay: DayOfWeek = .monday
  @Published public private(set) var watchZoneCount: Int = 0
  @Published public internal(set) var error: DomainError?
  private var cachedServerProfile: ServerProfile?
  private var pushEnabled: Bool = true   // round-tripped, not user-facing
}
```

- `load()`: fetch profile + zone count in parallel (`userProfileRepository.create()` + `watchZoneRepository.loadAll().count`).
- Four setters: `setSavedDecisionPush(_:)`, `setSavedDecisionEmail(_:)`, `setEmailDigestEnabled(_:)`, `setDigestDay(_:)`. Each uses **optimistic update + rollback on failure**, mirroring `SettingsViewModel.persistSavedDecisionPreference` (lines 223-257). Every PATCH includes all five fields (unchanged values sourced from `cachedServerProfile`).
- Constructor injects `UserProfileRepository` + `WatchZoneRepository`.

**Tests** (`NotificationPreferencesViewModelTests.swift`, Swift Testing `@Suite`/`@Test`/`#expect`):
- `loadPopulatesFieldsFromProfile`
- `loadPopulatesWatchZoneCount`
- `loadFallsBackToDefaultsOnRepositoryThrow`
- `setSavedDecisionPushRoundTripsOtherFields`
- `setSavedDecisionEmailRoundTripsOtherFields`
- `setEmailDigestEnabledRoundTripsOtherFields`
- `setDigestDayRoundTripsOtherFields`
- `failedUpdateRollsBackOptimisticChange`
- `failedUpdatePopulatesError`

Add `SpyWatchZoneRepository.swift` in `town-crier-tests/Sources/Spies/` if one doesn't already exist.

**Files (Phase 1):**
- `mobile/ios/packages/town-crier-presentation/Sources/Features/NotificationPreferences/NotificationPreferencesViewModel.swift` (new)
- `mobile/ios/town-crier-tests/Sources/Features/NotificationPreferencesViewModelTests.swift` (new)
- `mobile/ios/town-crier-tests/Sources/Spies/SpyWatchZoneRepository.swift` (new if missing)
- `mobile/ios/packages/town-crier-domain/Sources/ValueObjects/DayOfWeek.swift` — add `displayName: String` computed property if missing

---

## Phase 2 — View + wiring

**Bead: tc-notifprefs-view** (depends on Phase 1)

### New screen (`NotificationPreferencesView.swift`)

`Form` with:
- **Section 1 — Saved Applications**: `Toggle` for `savedDecisionPush`, `Toggle` for `savedDecisionEmail`. Footer: "Get notified when there's a decision on an application you've saved."
- **Section 2 — Email Digest**: `Toggle` for `emailDigestEnabled`, `Picker` for `digestDay` (disabled when digest is off). Footer: "A weekly summary of new applications in your watched zones."
- **Section 3 — Watch Zones**: Navigation row showing `"\(watchZoneCount) zones"` / `"No zones yet"`. Tapping fires `onZonesTap()`.
- **Footer link**: `"Open iOS notification settings"` (SF Symbol `gearshape`), fires `onSystemSettingsTap()`. Caption: "Banner style, sounds, badges, and Focus modes are managed by iOS."
- **Error section**: if `viewModel.error != nil`, render `DomainError.userMessage`, mirroring `ZonePreferencesView.swift:91-102`.

Callbacks: `onZonesTap: (() -> Void)?`, `onSystemSettingsTap: (() -> Void)?`

Accessibility labels: `"Saved applications — push"`, `"Saved applications — email"`, `"Weekly digest"`, `"Digest day"`, `"Open iOS notification settings"`.

### AppCoordinator changes

`mobile/ios/packages/town-crier-presentation/Sources/Coordinators/AppCoordinator.swift`:
- Add `@Published public var isNotificationPreferencesPresented = false`.
- Add `public func showNotificationPreferences() { isNotificationPreferencesPresented = true }`.
- Add `public func makeNotificationPreferencesViewModel() -> NotificationPreferencesViewModel` (inject repos).
- Add a "switch to Zones tab" hook (or expose a `selectedTab` binding the View layer can write to).
- Keep `showSystemNotificationSettings()` + `isOpeningSystemNotificationSettings` unchanged.

### TownCrierApp.swift changes

- Change `onNotificationPreferences` from `coordinator.showSystemNotificationSettings()` → `coordinator.showNotificationPreferences()`.
- Add `.navigationDestination(isPresented: $coordinator.isNotificationPreferencesPresented)` on the Settings `NavigationStack` to push `NotificationPreferencesView` (first use of `.navigationDestination` in the codebase — push pattern, not sheet-on-sheet).
- Wire `onSystemSettingsTap` → `coordinator.showSystemNotificationSettings()`.
- Wire `onZonesTap` → dismiss Settings + switch to Zones tab.

### SettingsView removals

`mobile/ios/packages/town-crier-presentation/Sources/Features/Settings/SettingsView.swift`:
- Delete `savedApplicationsSection` (lines ~49, ~183-212).
- Delete `requestNotificationPreferences()` test seam (lines ~40-42) if no longer needed.

### SettingsViewModel removals

`mobile/ios/packages/town-crier-presentation/Sources/Features/Settings/SettingsViewModel.swift`:
- Remove `savedDecisionPush`, `savedDecisionEmail`, their setters, `loadSavedDecisionPreferences`, `persistSavedDecisionPreference`, `cachedServerProfile`.
- Remove the `loadSavedDecisionPreferences()` call from `load()`.
- Strip corresponding `clearSession()` lines.

### Test updates

- `NotificationPreferencesViewTests.swift` (new): construction + callback wiring, mirrors `SettingsViewTests`.
- `AppCoordinatorTests.swift`: add `showNotificationPreferences_setsFlagToTrue`.
- `SettingsViewModelTests.swift`: delete tests for removed published fields.
- `SettingsViewModelSavedDecisionTests.swift`: relocate tests to `NotificationPreferencesViewModelTests`; delete this file.
- `SettingsViewTests.swift`: update/drop `notificationPreferencesCallback_isInvokedOnRequest`.

**Files (Phase 2):**
- `mobile/ios/packages/town-crier-presentation/Sources/Features/NotificationPreferences/NotificationPreferencesView.swift` (new)
- `mobile/ios/packages/town-crier-presentation/Sources/Coordinators/AppCoordinator.swift`
- `mobile/ios/town-crier-app/Sources/TownCrierApp.swift`
- `mobile/ios/packages/town-crier-presentation/Sources/Features/Settings/SettingsView.swift`
- `mobile/ios/packages/town-crier-presentation/Sources/Features/Settings/SettingsViewModel.swift`
- `mobile/ios/town-crier-tests/Sources/Features/NotificationPreferencesViewTests.swift` (new)
- `mobile/ios/town-crier-tests/Sources/Features/AppCoordinatorTests.swift`
- `mobile/ios/town-crier-tests/Sources/Features/SettingsViewModelTests.swift`
- `mobile/ios/town-crier-tests/Sources/Features/SettingsViewModelSavedDecisionTests.swift` (delete)
- `mobile/ios/town-crier-tests/Sources/Features/SettingsViewTests.swift`

---

## Out of Scope

- Saved-app notification trigger granularity (tc-ah9c, deferred)
- APNs push pipeline (tc-fqun)
- Quiet hours / DND inside the app
- `pushEnabled` user-facing toggle (round-tripped silently)
- Web parity
- API or domain changes

## Related

- tc-ah9c: Saved-app notifications trigger granularity (deferred)
- tc-fqun: APNs push notifications epic
- Design reference: `ZonePreferencesView.swift` (optimistic pattern + error section)
- Code reference: `SettingsViewModel.persistSavedDecisionPreference` lines 223-257
