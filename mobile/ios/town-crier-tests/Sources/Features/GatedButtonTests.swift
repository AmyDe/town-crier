import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("GatedButton")
@MainActor
struct GatedButtonTests {

  @Test func init_entitled_rendersWithoutCrashing() {
    let gate = FeatureGate(tier: .pro)
    let sut = GatedButton(
      label: "Search",
      entitlement: .searchApplications,
      featureGate: gate,
      action: {},
      onUpgradeRequired: {}
    )
    _ = sut.body
  }

  @Test func init_notEntitled_rendersWithoutCrashing() {
    let gate = FeatureGate(tier: .free)
    let sut = GatedButton(
      label: "Search",
      entitlement: .searchApplications,
      featureGate: gate,
      action: {},
      onUpgradeRequired: {}
    )
    _ = sut.body
  }

  @Test func isEnabled_proUser_search_returnsTrue() {
    let gate = FeatureGate(tier: .pro)
    let sut = GatedButton(
      label: "Search",
      entitlement: .searchApplications,
      featureGate: gate,
      action: {},
      onUpgradeRequired: {}
    )

    #expect(sut.isEnabled)
  }

  @Test func isEnabled_freeUser_search_returnsFalse() {
    let gate = FeatureGate(tier: .free)
    let sut = GatedButton(
      label: "Search",
      entitlement: .searchApplications,
      featureGate: gate,
      action: {},
      onUpgradeRequired: {}
    )

    #expect(!sut.isEnabled)
  }

  @Test func isEnabled_personalUser_search_returnsFalse() {
    let gate = FeatureGate(tier: .personal)
    let sut = GatedButton(
      label: "Search",
      entitlement: .searchApplications,
      featureGate: gate,
      action: {},
      onUpgradeRequired: {}
    )

    #expect(!sut.isEnabled)
  }
}
