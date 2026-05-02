import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("SubscriptionUpsellSheet")
@MainActor
struct SubscriptionUpsellSheetTests {

  // MARK: - Initialization

  @Test func init_withEntitlement_createsView() {
    let sut = SubscriptionUpsellSheet(
      entitlement: .statusChangeAlerts,
      onViewPlans: {},
      onDismiss: {}
    )
    _ = sut.body
  }

  @Test(arguments: Entitlement.allCases)
  func allEntitlements_renderWithoutCrashing(entitlement: Entitlement) {
    let sut = SubscriptionUpsellSheet(
      entitlement: entitlement,
      onViewPlans: {},
      onDismiss: {}
    )
    _ = sut.body
  }

  // MARK: - Callbacks

  @Test func onViewPlans_isCalled_whenViewPlansTapped() {
    var viewPlansCalled = false
    let sut = SubscriptionUpsellSheet(
      entitlement: .statusChangeAlerts,
      onViewPlans: { viewPlansCalled = true },
      onDismiss: {}
    )
    sut.simulateViewPlansTap()
    #expect(viewPlansCalled)
  }

  @Test func onDismiss_isCalled_whenNotNowTapped() {
    var dismissCalled = false
    let sut = SubscriptionUpsellSheet(
      entitlement: .decisionUpdateAlerts,
      onViewPlans: {},
      onDismiss: { dismissCalled = true }
    )
    sut.simulateDismissTap()
    #expect(dismissCalled)
  }
}
