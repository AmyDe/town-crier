import Foundation
import Testing
import TownCrierDomain

@Suite("PlanningApplication statusHistory")
struct StatusTimelineTests {

  @Test func statusHistory_defaultsToEmpty() {
    let app = PlanningApplication.pendingReview

    #expect(app.statusHistory.isEmpty)
  }

  @Test func statusHistory_preservesChronologicalEvents() {
    let received = StatusEvent(
      status: .undecided, date: Date(timeIntervalSince1970: 1_700_000_000))
    let decided = StatusEvent(status: .permitted, date: Date(timeIntervalSince1970: 1_700_100_000))
    let app = PlanningApplication(
      id: PlanningApplicationId("APP-010"),
      reference: ApplicationReference("2026/0300"),
      authority: LocalAuthority(code: "CAM", name: "Cambridge"),
      status: .permitted,
      receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
      description: "Test application",
      address: "1 Test Lane",
      statusHistory: [received, decided]
    )

    #expect(app.statusHistory.count == 2)
    #expect(app.statusHistory[0].status == .undecided)
    #expect(app.statusHistory[1].status == .permitted)
  }

  @Test func currentStatus_returnsLastEventStatus() {
    let received = StatusEvent(
      status: .undecided, date: Date(timeIntervalSince1970: 1_700_000_000))
    let decided = StatusEvent(status: .permitted, date: Date(timeIntervalSince1970: 1_700_100_000))
    let app = PlanningApplication(
      id: PlanningApplicationId("APP-010"),
      reference: ApplicationReference("2026/0300"),
      authority: LocalAuthority(code: "CAM", name: "Cambridge"),
      status: .permitted,
      receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
      description: "Test application",
      address: "1 Test Lane",
      statusHistory: [received, decided]
    )

    #expect(app.statusHistory.last?.status == .permitted)
  }
}
