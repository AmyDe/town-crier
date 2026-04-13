import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("WatchZoneUpsellView")
@MainActor
struct WatchZoneUpsellViewTests {

  // MARK: - Initialization

  @Test func init_createsView() {
    let sut = WatchZoneUpsellView(
      valueProposition: "Upgrade to monitor multiple areas.",
      onViewPlans: {},
      onDismiss: {}
    )
    _ = sut.body
  }

  // MARK: - Callbacks

  @Test func onViewPlans_isCalled_whenViewPlansTapped() {
    var viewPlansCalled = false
    let sut = WatchZoneUpsellView(
      valueProposition: "Upgrade to monitor multiple areas.",
      onViewPlans: { viewPlansCalled = true },
      onDismiss: {}
    )
    sut.simulateViewPlansTap()
    #expect(viewPlansCalled)
  }

  @Test func onDismiss_isCalled_whenNotNowTapped() {
    var dismissCalled = false
    let sut = WatchZoneUpsellView(
      valueProposition: "Upgrade to monitor multiple areas.",
      onViewPlans: {},
      onDismiss: { dismissCalled = true }
    )
    sut.simulateDismissTap()
    #expect(dismissCalled)
  }
}
