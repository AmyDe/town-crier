import Foundation
import Testing

@testable import TownCrierDomain
@testable import TownCrierPresentation

/// The instant push/email toggles in the watch-zone editor are paid-only (tc-bd6i).
/// Free-tier users see them locked with an upsell rather than as functioning controls,
/// because the server never delivers instant alerts to free accounts.
@MainActor
@Suite("WatchZoneEditorViewModel — instant-alert gating")
struct WatchZoneEditorViewModelGatingTests {
  private var spyGeocoder: SpyPostcodeGeocoder!
  private var spyRepository: SpyWatchZoneRepository!

  init() {
    spyGeocoder = SpyPostcodeGeocoder()
    spyRepository = SpyWatchZoneRepository()
  }

  private func makeViewModel(tier: SubscriptionTier) -> WatchZoneEditorViewModel {
    WatchZoneEditorViewModel(
      geocoder: spyGeocoder,
      repository: spyRepository,
      tier: tier
    )
  }

  // MARK: - featureGate reflects the tier

  @Test func featureGate_carriesTheTier_free() {
    #expect(makeViewModel(tier: .free).featureGate.tier == .free)
  }

  @Test func featureGate_carriesTheTier_personal() {
    #expect(makeViewModel(tier: .personal).featureGate.tier == .personal)
  }

  @Test func featureGate_carriesTheTier_pro() {
    #expect(makeViewModel(tier: .pro).featureGate.tier == .pro)
  }

  // MARK: - instant-alert entitlement gates the toggles

  @Test func freeTier_doesNotGrantInstantAlertEntitlement() {
    let sut = makeViewModel(tier: .free)
    #expect(!sut.featureGate.hasEntitlement(sut.instantAlertEntitlement))
  }

  @Test func personalTier_grantsInstantAlertEntitlement() {
    let sut = makeViewModel(tier: .personal)
    #expect(sut.featureGate.hasEntitlement(sut.instantAlertEntitlement))
  }

  @Test func proTier_grantsInstantAlertEntitlement() {
    let sut = makeViewModel(tier: .pro)
    #expect(sut.featureGate.hasEntitlement(sut.instantAlertEntitlement))
  }

  // MARK: - The notifications section is always present now

  @Test func notificationsSection_alwaysVisible_free() {
    #expect(makeViewModel(tier: .free).areNotificationTogglesVisible)
  }

  @Test func notificationsSection_alwaysVisible_personal() {
    #expect(makeViewModel(tier: .personal).areNotificationTogglesVisible)
  }

  // MARK: - Requesting an upgrade surfaces the in-editor upsell gate

  @Test func requestInstantAlertUpgrade_setsEntitlementGate() {
    let sut = makeViewModel(tier: .free)

    sut.requestInstantAlertUpgrade()

    #expect(sut.entitlementGate == sut.instantAlertEntitlement)
  }

  @Test func requestInstantAlertUpgrade_doesNotSetInlineError() {
    let sut = makeViewModel(tier: .free)

    sut.requestInstantAlertUpgrade()

    #expect(sut.error == nil)
  }

  // MARK: - The upsell's "View Plans" routes to the subscription screen

  @Test func viewPlans_invokesOnUpgradeRequired() {
    let sut = makeViewModel(tier: .free)
    var upgradeRequested = false
    sut.onUpgradeRequired = { upgradeRequested = true }

    sut.viewPlans()

    #expect(upgradeRequested)
  }
}
