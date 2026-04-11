import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("EntitlementGatingViewModel")
@MainActor
struct EntitlementGatingViewModelTests {

    // MARK: - Test helper conforming to the protocol

    final class TestGatingViewModel: EntitlementGatingViewModel {
        var error: DomainError?
        var entitlementGate: Entitlement?
    }

    // MARK: - insufficientEntitlement sets entitlementGate

    @Test func handleError_insufficientEntitlement_setsEntitlementGate() {
        let sut = TestGatingViewModel()

        sut.handleError(DomainError.insufficientEntitlement(required: "searchApplications"))

        #expect(sut.entitlementGate == .searchApplications)
        #expect(sut.error == nil)
    }

    @Test func handleError_insufficientEntitlement_mapsRequiredToEntitlement() {
        let sut = TestGatingViewModel()

        sut.handleError(DomainError.insufficientEntitlement(required: "statusChangeAlerts"))

        #expect(sut.entitlementGate == .statusChangeAlerts)
    }

    @Test func handleError_insufficientEntitlement_unknownRequired_setsError() {
        let sut = TestGatingViewModel()

        sut.handleError(
            DomainError.insufficientEntitlement(required: "unknownFeature")
        )

        #expect(sut.entitlementGate == nil)
        #expect(sut.error == .insufficientEntitlement(required: "unknownFeature"))
    }

    // MARK: - Non-entitlement errors fall through to normal handling

    @Test func handleError_networkError_setsErrorNotGate() {
        let sut = TestGatingViewModel()

        sut.handleError(DomainError.networkUnavailable)

        #expect(sut.error == .networkUnavailable)
        #expect(sut.entitlementGate == nil)
    }

    @Test func handleError_sessionExpired_setsErrorNotGate() {
        let sut = TestGatingViewModel()

        sut.handleError(DomainError.sessionExpired)

        #expect(sut.error == .sessionExpired)
        #expect(sut.entitlementGate == nil)
    }

    // MARK: - Overwrites previous gate

    @Test func handleError_overwritesPreviousGate() {
        let sut = TestGatingViewModel()
        sut.entitlementGate = .searchApplications

        sut.handleError(DomainError.insufficientEntitlement(required: "statusChangeAlerts"))

        #expect(sut.entitlementGate == .statusChangeAlerts)
    }

    @Test func handleError_nonEntitlementError_clearsGate() {
        let sut = TestGatingViewModel()
        sut.entitlementGate = .searchApplications

        sut.handleError(DomainError.networkUnavailable)

        #expect(sut.entitlementGate == nil)
        #expect(sut.error == .networkUnavailable)
    }
}
