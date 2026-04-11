import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ApplicationStatus+Display")
struct ApplicationStatusDisplayTests {

  // MARK: - displayLabel

  @Test func displayLabel_underReview_returnsPending() {
    #expect(ApplicationStatus.underReview.displayLabel == "Pending")
  }

  @Test func displayLabel_approved_returnsApproved() {
    #expect(ApplicationStatus.approved.displayLabel == "Approved")
  }

  @Test func displayLabel_refused_returnsRefused() {
    #expect(ApplicationStatus.refused.displayLabel == "Refused")
  }

  @Test func displayLabel_withdrawn_returnsWithdrawn() {
    #expect(ApplicationStatus.withdrawn.displayLabel == "Withdrawn")
  }

  @Test func displayLabel_appealed_returnsAppealed() {
    #expect(ApplicationStatus.appealed.displayLabel == "Appealed")
  }

  @Test func displayLabel_unknown_returnsUnknown() {
    #expect(ApplicationStatus.unknown.displayLabel == "Unknown")
  }

  // MARK: - displayIcon

  @Test func displayIcon_underReview_returnsClock() {
    #expect(ApplicationStatus.underReview.displayIcon == "clock")
  }

  @Test func displayIcon_approved_returnsCheckmarkCircle() {
    #expect(ApplicationStatus.approved.displayIcon == "checkmark.circle")
  }

  @Test func displayIcon_refused_returnsXmarkCircle() {
    #expect(ApplicationStatus.refused.displayIcon == "xmark.circle")
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
      .underReview, .approved, .refused, .withdrawn, .appealed, .unknown,
    ]
    for status in allStatuses {
      // Verify each status produces a color without crashing.
      // SwiftUI Color is not equatable in a test-friendly way,
      // so we verify the mapping is exhaustive and non-crashing.
      _ = status.displayColor
    }
  }
}
