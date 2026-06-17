import Foundation
import Testing

@testable import TownCrierDomain

@Suite("PlanningApplication domain logic")
struct PlanningApplicationTests {
  // MARK: - latestUnreadEvent (tc-1nsa.8)

  @Test("latestUnreadEvent defaults to nil when not provided")
  func init_defaultsLatestUnreadEventToNil() {
    let application = PlanningApplication.pendingReview
    #expect(application.latestUnreadEvent == nil)
  }

  @Test("latestUnreadEvent stores the supplied descriptor")
  func init_storesLatestUnreadEvent() {
    let event = LatestUnreadEvent(
      type: "DecisionUpdate",
      decision: "Permitted",
      createdAt: Date(timeIntervalSince1970: 1_712_000_000)
    )
    let application = PlanningApplication(
      id: PlanningApplicationId(authority: "CAM", name: "APP-001"),
      reference: ApplicationReference("2026/0042"),
      authority: LocalAuthority(code: "CAM", name: "Cambridge"),
      status: .undecided,
      receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
      description: "Test",
      address: "1 Test Street",
      latestUnreadEvent: event
    )
    #expect(application.latestUnreadEvent == event)
  }
}
