import Foundation
import Testing

@testable import TownCrierDomain

@Suite("PlanningApplication domain logic")
struct PlanningApplicationTests {
  @Test func markAsDecided_permitted_updatesStatus() throws {
    var application = PlanningApplication.pendingReview
    try application.markAsDecided(.permitted, on: Date())
    #expect(application.status == .permitted)
  }

  @Test func markAsDecided_conditions_updatesStatus() throws {
    var application = PlanningApplication.pendingReview
    try application.markAsDecided(.conditions, on: Date())
    #expect(application.status == .conditions)
  }

  @Test func markAsDecided_rejected_updatesStatus() throws {
    var application = PlanningApplication.pendingReview
    try application.markAsDecided(.rejected, on: Date())
    #expect(application.status == .rejected)
  }

  @Test func markAsDecided_whenAlreadyPermitted_throwsError() {
    var application = PlanningApplication.permitted
    #expect(throws: DomainError.invalidStatusTransition(from: .permitted, to: .rejected)) {
      try application.markAsDecided(.rejected, on: Date())
    }
  }

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
      id: PlanningApplicationId("APP-001"),
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
