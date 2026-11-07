import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("entitlementGateSheet modifier")
@MainActor
struct EntitlementGateModifierTests {

  // `.sheet(item:)` cannot have its `.body` called outside a SwiftUI render
  // pass, so these tests only check the modifier applies.

  @Test func modifier_canBeAppliedToView_withEntitlement() {
    var gate: Entitlement? = .statusChangeAlerts
    let binding = Binding(get: { gate }, set: { gate = $0 })
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
