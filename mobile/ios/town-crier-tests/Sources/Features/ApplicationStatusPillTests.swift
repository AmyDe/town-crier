import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests for ``ApplicationStatusPill`` — the status capsule rendered on
/// `ApplicationListRow`. The pill defers all vocabulary, icon, and colour
/// decisions to ``ApplicationStatus`` display extensions; read/unread state
/// is signalled by ``ApplicationListRow``'s leading dot, not by mutating
/// the pill itself.
@Suite("ApplicationStatusPill")
@MainActor
struct ApplicationStatusPillTests {

  // MARK: - Construction

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
    let sut = ApplicationStatusPill(status: status)
    _ = sut.body
  }

  // MARK: - Vocabulary delegation

  /// The pill must surface the same label as ``ApplicationStatus.displayLabel``
  /// rather than carrying its own copy of the UK vocabulary. This guards
  /// against the bug pattern that motivated this bead: ``NotificationDecisionBadge``
  /// drifted by maintaining a private decision table parallel to the canonical
  /// one on the domain enum.
  @Test(arguments: [
    ApplicationStatus.permitted,
    ApplicationStatus.conditions,
    ApplicationStatus.rejected,
    ApplicationStatus.appealed,
    ApplicationStatus.undecided,
  ])
  func label_matchesApplicationStatusDisplayLabel(status: ApplicationStatus) {
    let sut = ApplicationStatusPill(status: status)

    #expect(sut.label == status.displayLabel)
  }

  @Test(arguments: [
    ApplicationStatus.permitted,
    ApplicationStatus.conditions,
    ApplicationStatus.rejected,
    ApplicationStatus.appealed,
    ApplicationStatus.undecided,
  ])
  func icon_matchesApplicationStatusDisplayIcon(status: ApplicationStatus) {
    let sut = ApplicationStatusPill(status: status)

    #expect(sut.iconName == status.displayIcon)
  }
}
