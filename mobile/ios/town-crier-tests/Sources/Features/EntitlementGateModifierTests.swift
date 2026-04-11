import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("entitlementGateSheet modifier")
@MainActor
struct EntitlementGateModifierTests {

  // The `.sheet(item:)` modifier cannot have its `.body` called outside
  // a SwiftUI render pass without a fatal error. These tests verify the
  // modifier compiles and can be applied — the runtime sheet presentation
  // is validated by the SubscriptionUpsellSheet render tests.

  @Test func modifier_canBeAppliedToView_withEntitlement() {
    var gate: Entitlement? = .searchApplications
    let binding = Binding(get: { gate }, set: { gate = $0 })
    // Verifies the modifier compiles and returns an opaque View.
    _ = AnyView(
      Text("Hello")
        .entitlementGateSheet(entitlement: binding) {}
    )
  }

  @Test func modifier_canBeAppliedToView_withNilEntitlement() {
    var gate: Entitlement?
    let binding = Binding(get: { gate }, set: { gate = $0 })
    _ = AnyView(
      Text("Hello")
        .entitlementGateSheet(entitlement: binding) {}
    )
  }

  @Test(arguments: Entitlement.allCases)
  func modifier_acceptsEachEntitlement(entitlement: Entitlement) {
    var gate: Entitlement? = entitlement
    let binding = Binding(get: { gate }, set: { gate = $0 })
    _ = AnyView(
      Text("Hello")
        .entitlementGateSheet(entitlement: binding) {}
    )
  }
}
