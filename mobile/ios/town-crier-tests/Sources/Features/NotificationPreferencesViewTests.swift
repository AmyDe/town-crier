import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Construction + callback-wiring tests for `NotificationPreferencesView`.
/// Mirrors `SettingsViewTests` — exercises the test-only seams that invoke
/// the view's callbacks as if the user had tapped the corresponding row.
@Suite("NotificationPreferencesView")
@MainActor
struct NotificationPreferencesViewTests {

  private func makeViewModel() -> NotificationPreferencesViewModel {
    NotificationPreferencesViewModel(
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: SpyWatchZoneRepository(),
      notificationService: SpyNotificationService()
    )
  }

  // MARK: - Construction

  @Test("can be constructed without callbacks")
  func construction_withoutCallbacks_succeeds() {
    let vm = makeViewModel()

    let view = NotificationPreferencesView(viewModel: vm)

    _ = view
  }

  @Test("can be constructed with both callbacks")
  func construction_withCallbacks_succeeds() {
    let vm = makeViewModel()
    let noop: () -> Void = {}

    let view = NotificationPreferencesView(
      viewModel: vm,
      onZonesTap: noop,
      onSystemSettingsTap: noop
    )

    _ = view
  }

  // MARK: - Callback wiring

  @Test("forwards the watch-zones-row tap to the callback")
  func zonesCallback_isInvokedOnRequest() {
    let vm = makeViewModel()
    var tapped = false
    let handler: () -> Void = { tapped = true }
    let view = NotificationPreferencesView(viewModel: vm, onZonesTap: handler)

    view.requestZonesTap()

    #expect(tapped)
  }

  @Test("forwards the system-settings-link tap to the callback")
  func systemSettingsCallback_isInvokedOnRequest() {
    let vm = makeViewModel()
    var tapped = false
    let handler: () -> Void = { tapped = true }
    let view = NotificationPreferencesView(viewModel: vm, onSystemSettingsTap: handler)

    view.requestSystemSettingsTap()

    #expect(tapped)
  }
}
