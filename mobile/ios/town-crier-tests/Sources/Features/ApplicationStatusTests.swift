import Foundation
import Testing

@testable import TownCrierDomain

@Suite("ApplicationStatus PlanIt vocabulary")
struct ApplicationStatusTests {

  // MARK: - Raw values mirror PlanIt's wire vocabulary

  @Test func undecided_rawValueIsUndecided() {
    #expect(ApplicationStatus.undecided.rawValue == "Undecided")
  }

  @Test func permitted_rawValueIsPermitted() {
    #expect(ApplicationStatus.permitted.rawValue == "Permitted")
  }

  @Test func conditions_rawValueIsConditions() {
    #expect(ApplicationStatus.conditions.rawValue == "Conditions")
  }

  @Test func rejected_rawValueIsRejected() {
    #expect(ApplicationStatus.rejected.rawValue == "Rejected")
  }

  @Test func withdrawn_rawValueIsWithdrawn() {
    #expect(ApplicationStatus.withdrawn.rawValue == "Withdrawn")
  }

  @Test func appealed_rawValueIsAppealed() {
    #expect(ApplicationStatus.appealed.rawValue == "Appealed")
  }

  @Test func unresolved_rawValueIsUnresolved() {
    #expect(ApplicationStatus.unresolved.rawValue == "Unresolved")
  }

  @Test func referred_rawValueIsReferred() {
    #expect(ApplicationStatus.referred.rawValue == "Referred")
  }

  @Test func notAvailable_rawValueIsNotAvailable() {
    #expect(ApplicationStatus.notAvailable.rawValue == "Not Available")
  }

  // MARK: - Decoding from PlanIt strings

  @Test(arguments: [
    ("Undecided", ApplicationStatus.undecided),
    ("Permitted", ApplicationStatus.permitted),
    ("Conditions", ApplicationStatus.conditions),
    ("Rejected", ApplicationStatus.rejected),
    ("Withdrawn", ApplicationStatus.withdrawn),
    ("Appealed", ApplicationStatus.appealed),
    ("Unresolved", ApplicationStatus.unresolved),
    ("Referred", ApplicationStatus.referred),
    ("Not Available", ApplicationStatus.notAvailable),
  ])
  func initFromRawValue_decodesPlanItVocabulary(raw: String, expected: ApplicationStatus) {
    #expect(ApplicationStatus(rawValue: raw) == expected)
  }

  @Test func initFromRawValue_unknownString_returnsNil() {
    #expect(ApplicationStatus(rawValue: "Mystery") == nil)
  }
}
