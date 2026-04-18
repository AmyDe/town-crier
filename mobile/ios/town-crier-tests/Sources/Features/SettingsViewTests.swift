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

  // MARK: - View Construction

  @Test("SettingsView can be constructed without the offer-code callback")
  func construction_withoutOfferCodeCallback_succeeds() {
    let vm = makeViewModel()

    let view = SettingsView(viewModel: vm)

    _ = view
  }

  @Test("SettingsView can be constructed with the offer-code callback")
  func construction_withOfferCodeCallback_succeeds() {
    let vm = makeViewModel()
    let noop: () -> Void = {}

    let view = SettingsView(viewModel: vm, onRedeemOfferCode: noop)

    _ = view
  }

  @Test("SettingsView forwards the redeem-offer-code tap to the callback")
  func redeemOfferCodeCallback_isInvokedOnRequest() {
    let vm = makeViewModel()
    var tapped = false
    let handler: () -> Void = { tapped = true }
    let view = SettingsView(viewModel: vm, onRedeemOfferCode: handler)

    view.requestRedeemOfferCode()

    #expect(tapped)
  }
}
