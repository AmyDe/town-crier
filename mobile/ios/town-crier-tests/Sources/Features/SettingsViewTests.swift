import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("SettingsView")
@MainActor
struct SettingsViewTests {

  // MARK: - Helpers

  private func makeViewModel() -> SettingsViewModel {
    SettingsViewModel(
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      appVersionProvider: SpyAppVersionProvider(),
      notificationService: SpyNotificationService(),
      defaults: UserDefaults(suiteName: UUID().uuidString) ?? .standard
    )
  }

  private func makeLifetimeCapableViewModel(
    entitlement: SubscriptionEntitlement
  ) -> SettingsViewModel {
    let authSpy = SpyAuthenticationService()
    authSpy.currentSessionResult = .valid
    let subscriptionSpy = SpySubscriptionService()
    subscriptionSpy.currentEntitlementResult = entitlement
    let profileSpy = SpyUserProfileRepository()
    profileSpy.createResult = .success(.proUser)
    return SettingsViewModel(
      authService: authSpy,
      subscriptionService: subscriptionSpy,
      userProfileRepository: profileSpy,
      appVersionProvider: SpyAppVersionProvider(),
      notificationService: SpyNotificationService(),
      defaults: UserDefaults(suiteName: UUID().uuidString) ?? .standard
    )
  }

  // MARK: - View Construction

  @Test("SettingsView can be constructed without the view-plans callback")
  func construction_withoutViewPlansCallback_succeeds() {
    let vm = makeViewModel()

    let view = SettingsView(viewModel: vm)

    _ = view
  }

  @Test("SettingsView forwards the view-plans tap to the callback")
  func viewPlansCallback_isInvokedOnRequest() {
    let vm = makeViewModel()
    var tapped = false
    let handler: () -> Void = { tapped = true }
    let view = SettingsView(viewModel: vm, onViewPlans: handler)

    view.requestViewPlans()

    #expect(tapped)
  }

  @Test("SettingsView shows the view-plans row when the user is not a lifetime holder")
  func showsViewPlansRow_whenNotLifetime_isTrue() async {
    let vm = makeLifetimeCapableViewModel(entitlement: .proMonthlyActive)
    await vm.load()
    let view = SettingsView(viewModel: vm)

    #expect(view.showsViewPlansRow)
  }

  @Test("SettingsView hides the view-plans row for a lifetime holder")
  func showsViewPlansRow_whenLifetime_isFalse() async {
    let vm = makeLifetimeCapableViewModel(entitlement: .proLifetime)
    await vm.load()
    let view = SettingsView(viewModel: vm)

    #expect(!view.showsViewPlansRow)
  }

  @Test("SettingsView forwards the notification-preferences tap to the callback")
  func notificationPreferencesCallback_isInvokedOnRequest() {
    let vm = makeViewModel()
    var tapped = false
    let handler: () -> Void = { tapped = true }
    let view = SettingsView(viewModel: vm, onNotificationPreferences: handler)

    view.requestNotificationPreferences()

    #expect(tapped)
  }

  @Test("SettingsView forwards the rate-app tap to the callback")
  func rateAppCallback_isInvokedOnRequest() {
    let vm = makeViewModel()
    var tapped = false
    let handler: () -> Void = { tapped = true }
    let view = SettingsView(viewModel: vm, onRateApp: handler)

    view.requestRateApp()

    #expect(tapped)
  }

  @Test("SettingsView export-data tap drives the ViewModel export flow")
  func exportDataTap_invokesViewModelExport() async {
    let profileSpy = SpyUserProfileRepository()
    let vm = SettingsViewModel(
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: profileSpy,
      appVersionProvider: SpyAppVersionProvider(),
      notificationService: SpyNotificationService(),
      defaults: UserDefaults(suiteName: UUID().uuidString) ?? .standard
    )
    let view = SettingsView(viewModel: vm)

    await view.requestExportData()

    #expect(profileSpy.exportDataCallCount == 1)
  }
}
