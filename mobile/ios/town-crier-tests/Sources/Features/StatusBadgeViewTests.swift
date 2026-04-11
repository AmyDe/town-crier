import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("StatusBadgeView")
@MainActor
struct StatusBadgeViewTests {

  // MARK: - Initialization

  @Test func init_withStatus_createsView() {
    let sut = StatusBadgeView(status: .underReview)
    _ = sut.body
  }

  @Test func init_withApprovedStatus_createsView() {
    let sut = StatusBadgeView(status: .approved)
    _ = sut.body
  }

  @Test func init_withRefusedStatus_createsView() {
    let sut = StatusBadgeView(status: .refused)
    _ = sut.body
  }

  @Test func init_withWithdrawnStatus_createsView() {
    let sut = StatusBadgeView(status: .withdrawn)
    _ = sut.body
  }

  @Test func init_withAppealedStatus_createsView() {
    let sut = StatusBadgeView(status: .appealed)
    _ = sut.body
  }

  @Test func init_withUnknownStatus_createsView() {
    let sut = StatusBadgeView(status: .unknown)
    _ = sut.body
  }

  // MARK: - All statuses render without crashing

  @Test(arguments: [
    ApplicationStatus.underReview,
    ApplicationStatus.approved,
    ApplicationStatus.refused,
    ApplicationStatus.withdrawn,
    ApplicationStatus.appealed,
    ApplicationStatus.unknown,
  ])
  func allStatuses_renderWithoutCrashing(status: ApplicationStatus) {
    let sut = StatusBadgeView(status: status)
    _ = sut.body
  }
}
