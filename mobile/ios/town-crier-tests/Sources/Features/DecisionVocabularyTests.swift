import Foundation
import Testing
import TownCrierDomain

/// Tests for ``DecisionVocabulary`` — the UK-vocabulary mapping that turns
/// PlanIt's wire `app_state` strings into the display terms residents
/// recognise. Mirrors the API-side `UkPlanningVocabulary` helper so push
/// payloads, in-app rows, and any future shared rendering stay in sync.
@Suite("DecisionVocabulary")
struct DecisionVocabularyTests {
  @Test func displayLabel_forPermitted_returnsApproved() {
    #expect(DecisionVocabulary.displayLabel(forPlanItAppState: "Permitted") == "Approved")
  }

  @Test func displayLabel_forConditions_returnsApprovedWithConditions() {
    #expect(
      DecisionVocabulary.displayLabel(forPlanItAppState: "Conditions") == "Approved with conditions"
    )
  }

  @Test func displayLabel_forRejected_returnsRefused() {
    #expect(DecisionVocabulary.displayLabel(forPlanItAppState: "Rejected") == "Refused")
  }

  @Test func displayLabel_forAppealed_returnsRefusalAppealed() {
    #expect(DecisionVocabulary.displayLabel(forPlanItAppState: "Appealed") == "Refusal appealed")
  }

  @Test func displayLabel_isCaseInsensitive() {
    #expect(DecisionVocabulary.displayLabel(forPlanItAppState: "permitted") == "Approved")
    #expect(DecisionVocabulary.displayLabel(forPlanItAppState: "REJECTED") == "Refused")
  }

  @Test func displayLabel_forUnrecognisedState_returnsNil() {
    #expect(DecisionVocabulary.displayLabel(forPlanItAppState: "Undecided") == nil)
    #expect(DecisionVocabulary.displayLabel(forPlanItAppState: "Withdrawn") == nil)
  }

  @Test func displayLabel_forNilOrBlank_returnsNil() {
    #expect(DecisionVocabulary.displayLabel(forPlanItAppState: nil) == nil)
    #expect(DecisionVocabulary.displayLabel(forPlanItAppState: "") == nil)
    #expect(DecisionVocabulary.displayLabel(forPlanItAppState: "   ") == nil)
  }
}
