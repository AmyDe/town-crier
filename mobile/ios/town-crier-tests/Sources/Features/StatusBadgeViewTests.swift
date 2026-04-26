import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("StatusBadgeView")
@MainActor
struct StatusBadgeViewTests {

  // MARK: - Initialization

  @Test func init_withStatus_createsView() {
    let sut = StatusBadgeView(status: .undecided)
    _ = sut.body
  }

  @Test func init_withPermittedStatus_createsView() {
    let sut = StatusBadgeView(status: .permitted)
    _ = sut.body
  }

  @Test func init_withConditionsStatus_createsView() {
    let sut = StatusBadgeView(status: .conditions)
    _ = sut.body
  }

  @Test func init_withRejectedStatus_createsView() {
    let sut = StatusBadgeView(status: .rejected)
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
    ApplicationStatus.undecided,
    ApplicationStatus.notAvailable,
    ApplicationStatus.permitted,
    ApplicationStatus.conditions,
    ApplicationStatus.rejected,
    ApplicationStatus.withdrawn,
    ApplicationStatus.appealed,
    ApplicationStatus.unresolved,
    ApplicationStatus.referred,
    ApplicationStatus.unknown,
  ])
  func allStatuses_renderWithoutCrashing(status: ApplicationStatus) {
    let sut = StatusBadgeView(status: status)
    _ = sut.body
  }
}
