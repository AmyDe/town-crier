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

  // MARK: - Permission section

  @Test("hidesSystemSettingsLink computes from VM authorization status")
  func hidesSystemSettingsLink_whenNotDetermined() async {
    let profileSpy = SpyUserProfileRepository()
    profileSpy.createResult = .success(.freeUser)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([])
    let notificationSpy = SpyNotificationService()
    notificationSpy.nextAuthorizationStatus = .notDetermined
    let vm = NotificationPreferencesViewModel(
      userProfileRepository: profileSpy,
      watchZoneRepository: zoneSpy,
      notificationService: notificationSpy
    )
    await vm.load()
    let view = NotificationPreferencesView(viewModel: vm)

    #expect(view.shouldHideSystemSettingsLink)
  }

  @Test("system settings link visible when denied")
  func systemSettingsLink_visibleWhenDenied() async {
    let profileSpy = SpyUserProfileRepository()
    profileSpy.createResult = .success(.freeUser)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([])
    let notificationSpy = SpyNotificationService()
    notificationSpy.nextAuthorizationStatus = .denied
    let vm = NotificationPreferencesViewModel(
      userProfileRepository: profileSpy,
      watchZoneRepository: zoneSpy,
      notificationService: notificationSpy
    )
    await vm.load()
    let view = NotificationPreferencesView(viewModel: vm)

    #expect(!view.shouldHideSystemSettingsLink)
  }

  @Test("system settings link visible when authorized")
  func systemSettingsLink_visibleWhenAuthorized() async {
    let profileSpy = SpyUserProfileRepository()
    profileSpy.createResult = .success(.freeUser)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([])
    let notificationSpy = SpyNotificationService()
    notificationSpy.nextAuthorizationStatus = .authorized
    let vm = NotificationPreferencesViewModel(
      userProfileRepository: profileSpy,
      watchZoneRepository: zoneSpy,
      notificationService: notificationSpy
    )
    await vm.load()
    let view = NotificationPreferencesView(viewModel: vm)

    #expect(!view.shouldHideSystemSettingsLink)
  }

  @Test("system settings link visible while status still loading")
  func systemSettingsLink_visibleWhenStatusNil() {
    let vm = makeViewModel()
    let view = NotificationPreferencesView(viewModel: vm)

    #expect(!view.shouldHideSystemSettingsLink)
  }

  @Test("requestPermissionButtonTap invokes VM requestPermission")
  func requestPermissionButtonTap_invokesViewModel() async {
    let profileSpy = SpyUserProfileRepository()
    profileSpy.createResult = .success(.freeUser)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([])
    let notificationSpy = SpyNotificationService()
    notificationSpy.nextAuthorizationStatus = .notDetermined
    let vm = NotificationPreferencesViewModel(
      userProfileRepository: profileSpy,
      watchZoneRepository: zoneSpy,
      notificationService: notificationSpy
    )
    await vm.load()
    let view = NotificationPreferencesView(viewModel: vm)

    await view.requestPermissionButtonTap()

    #expect(notificationSpy.requestPermissionCallCount == 1)
  }

  @Test("scenePhase becoming active refreshes authorization status")
  func scenePhaseActive_refreshesAuthorizationStatus() async {
    let profileSpy = SpyUserProfileRepository()
    profileSpy.createResult = .success(.freeUser)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([])
    let notificationSpy = SpyNotificationService()
    notificationSpy.nextAuthorizationStatus = .denied
    let vm = NotificationPreferencesViewModel(
      userProfileRepository: profileSpy,
      watchZoneRepository: zoneSpy,
      notificationService: notificationSpy
    )
    await vm.load()
    let callsAfterLoad = notificationSpy.authorizationStatusCallCount
    notificationSpy.nextAuthorizationStatus = .authorized
    let view = NotificationPreferencesView(viewModel: vm)

    await view.requestScenePhaseActive()

    #expect(notificationSpy.authorizationStatusCallCount == callsAfterLoad + 1)
    #expect(vm.authorizationStatus == .authorized)
  }
}
