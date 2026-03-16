import Foundation
import Testing

@testable import TownCrierDomain

@Suite("PlanningApplication domain logic")
struct PlanningApplicationTests {
    @Test func markAsDecided_approved_updatesStatus() throws {
        var application = PlanningApplication.pendingReview
        try application.markAsDecided(.approved, on: Date())
        #expect(application.status == .approved)
    }

    @Test func markAsDecided_refused_updatesStatus() throws {
        var application = PlanningApplication.pendingReview
        try application.markAsDecided(.refused, on: Date())
        #expect(application.status == .refused)
    }

    @Test func markAsDecided_whenAlreadyApproved_throwsError() {
        var application = PlanningApplication.approved
        #expect(throws: DomainError.invalidStatusTransition(from: .approved, to: .refused)) {
            try application.markAsDecided(.refused, on: Date())
        }
    }
}
