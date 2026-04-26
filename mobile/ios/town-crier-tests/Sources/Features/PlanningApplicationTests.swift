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
}
