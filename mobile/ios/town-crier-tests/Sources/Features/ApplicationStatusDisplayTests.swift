import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ApplicationStatus+Display")
struct ApplicationStatusDisplayTests {

  // MARK: - displayLabel

  @Test func displayLabel_undecided_returnsPending() {
    #expect(ApplicationStatus.undecided.displayLabel == "Pending")
  }

  @Test func displayLabel_notAvailable_returnsNotAvailable() {
    #expect(ApplicationStatus.notAvailable.displayLabel == "Not Available")
  }

  @Test func displayLabel_permitted_returnsGranted() {
    #expect(ApplicationStatus.permitted.displayLabel == "Granted")
  }

  @Test func displayLabel_conditions_returnsGrantedWithConditions() {
    #expect(ApplicationStatus.conditions.displayLabel == "Granted with conditions")
  }

  @Test func displayLabel_rejected_returnsRefused() {
    #expect(ApplicationStatus.rejected.displayLabel == "Refused")
  }

  @Test func displayLabel_withdrawn_returnsWithdrawn() {
    #expect(ApplicationStatus.withdrawn.displayLabel == "Withdrawn")
  }

  @Test func displayLabel_appealed_returnsAppealed() {
    #expect(ApplicationStatus.appealed.displayLabel == "Appealed")
  }

  @Test func displayLabel_unresolved_returnsUnresolved() {
    #expect(ApplicationStatus.unresolved.displayLabel == "Unresolved")
  }

  @Test func displayLabel_referred_returnsReferred() {
    #expect(ApplicationStatus.referred.displayLabel == "Referred")
  }

  @Test func displayLabel_unknown_returnsUnknown() {
    #expect(ApplicationStatus.unknown.displayLabel == "Unknown")
  }

  // MARK: - displayIcon

  @Test func displayIcon_undecided_returnsClock() {
    #expect(ApplicationStatus.undecided.displayIcon == "clock")
  }

  @Test func displayIcon_notAvailable_returnsMinusCircle() {
    #expect(ApplicationStatus.notAvailable.displayIcon == "minus.circle")
  }

  @Test func displayIcon_permitted_returnsCheckmarkCircle() {
    #expect(ApplicationStatus.permitted.displayIcon == "checkmark.circle")
  }

  @Test func displayIcon_conditions_returnsCheckmarkCircleBadgeQuestionmark() {
    #expect(ApplicationStatus.conditions.displayIcon == "checkmark.circle.badge.questionmark")
  }

  @Test func displayIcon_rejected_returnsXmarkCircle() {
    #expect(ApplicationStatus.rejected.displayIcon == "xmark.circle")
  }

  @Test func displayIcon_withdrawn_returnsArrowUturnBackwardCircle() {
    #expect(ApplicationStatus.withdrawn.displayIcon == "arrow.uturn.backward.circle")
  }

  @Test func displayIcon_appealed_returnsExclamationmarkTriangle() {
    #expect(ApplicationStatus.appealed.displayIcon == "exclamationmark.triangle")
  }

  @Test func displayIcon_unknown_returnsQuestionmarkCircle() {
    #expect(ApplicationStatus.unknown.displayIcon == "questionmark.circle")
  }

  // MARK: - displayColor (smoke test)

  @Test func displayColor_allStatusesReturnColor() {
    let allStatuses: [ApplicationStatus] = [
      .undecided, .permitted, .conditions, .rejected, .withdrawn, .appealed,
      .unresolved, .referred, .notAvailable, .unknown,
    ]
    for status in allStatuses {
      // Verify each status produces a color without crashing.
      // SwiftUI Color is not equatable in a test-friendly way,
      // so we verify the mapping is exhaustive and non-crashing.
      _ = status.displayColor
    }
  }
}
