import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("GatedToggle")
@MainActor
struct GatedToggleTests {

  @Test func init_entitled_rendersWithoutCrashing() {
    let gate = FeatureGate(tier: .personal)
    let sut = GatedToggle(
      label: "Status Changes",
      isOn: .constant(true),
      entitlement: .statusChangeAlerts,
      featureGate: gate
    ) {}
    _ = sut.body
  }

  @Test func init_notEntitled_rendersWithoutCrashing() {
    let gate = FeatureGate(tier: .free)
    let sut = GatedToggle(
      label: "Status Changes",
      isOn: .constant(false),
      entitlement: .statusChangeAlerts,
      featureGate: gate
    ) {}
    _ = sut.body
  }

  @Test func isEnabled_personalUser_statusChanges_returnsTrue() {
    let gate = FeatureGate(tier: .personal)
    let sut = GatedToggle(
      label: "Status Changes",
      isOn: .constant(true),
      entitlement: .statusChangeAlerts,
      featureGate: gate
    ) {}

    #expect(sut.isEnabled)
  }

  @Test func isEnabled_freeUser_statusChanges_returnsFalse() {
    let gate = FeatureGate(tier: .free)
    let sut = GatedToggle(
      label: "Status Changes",
      isOn: .constant(false),
      entitlement: .statusChangeAlerts,
      featureGate: gate
    ) {}

    #expect(!sut.isEnabled)
  }

  @Test func isEnabled_proUser_decisionUpdateAlerts_returnsTrue() {
    let gate = FeatureGate(tier: .pro)
    let sut = GatedToggle(
      label: "Decision Updates",
      isOn: .constant(true),
      entitlement: .decisionUpdateAlerts,
      featureGate: gate
    ) {}

    #expect(sut.isEnabled)
  }
}
