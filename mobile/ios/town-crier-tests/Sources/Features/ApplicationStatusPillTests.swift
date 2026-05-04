import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests for ``ApplicationStatusPill`` — the saturation-aware status capsule
/// rendered on `ApplicationListRow`. The pill defers all vocabulary, icon,
/// and colour decisions to ``ApplicationStatus`` display extensions; what
/// is unique here is the muted/saturated rendering knob driven by whether
/// the row has an unread event.
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
  func allStatuses_saturated_renderWithoutCrashing(status: ApplicationStatus) {
    let sut = ApplicationStatusPill(status: status, isMuted: false)
    _ = sut.body
  }

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
  func allStatuses_muted_renderWithoutCrashing(status: ApplicationStatus) {
    let sut = ApplicationStatusPill(status: status, isMuted: true)
    _ = sut.body
  }

  // MARK: - Defaults

  @Test("defaults to saturated (isMuted == false) when not specified")
  func init_defaultsToSaturated() {
    let sut = ApplicationStatusPill(status: .permitted)

    #expect(!sut.isMuted)
  }

  // MARK: - Saturation knob

  @Test("isMuted reflects the constructor argument")
  func isMuted_reflectsInput() {
    let saturated = ApplicationStatusPill(status: .permitted, isMuted: false)
    let muted = ApplicationStatusPill(status: .permitted, isMuted: true)

    #expect(!saturated.isMuted)
    #expect(muted.isMuted)
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
