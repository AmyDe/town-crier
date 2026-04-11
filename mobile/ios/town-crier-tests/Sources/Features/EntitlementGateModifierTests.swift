import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("entitlementGateSheet modifier")
@MainActor
struct EntitlementGateModifierTests {

    @Test func modifier_canBeAppliedToView() {
        var gate: Entitlement? = .searchApplications
        let binding = Binding(get: { gate }, set: { gate = $0 })
        let sut = Text("Hello")
            .entitlementGateSheet(entitlement: binding) {}
        _ = sut.body
    }

    @Test func modifier_withNilEntitlement_rendersHost() {
        var gate: Entitlement?
        let binding = Binding(get: { gate }, set: { gate = $0 })
        let sut = Text("Hello")
            .entitlementGateSheet(entitlement: binding) {}
        _ = sut.body
    }

    @Test(arguments: Entitlement.allCases)
    func modifier_withEachEntitlement_rendersWithoutCrashing(entitlement: Entitlement) {
        var gate: Entitlement? = entitlement
        let binding = Binding(get: { gate }, set: { gate = $0 })
        let sut = Text("Hello")
            .entitlementGateSheet(entitlement: binding) {}
        _ = sut.body
    }
}
