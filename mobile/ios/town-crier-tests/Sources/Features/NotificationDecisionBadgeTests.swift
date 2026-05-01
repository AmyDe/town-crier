import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("NotificationDecisionBadge")
struct NotificationDecisionBadgeTests {

  // MARK: - Helpers

  private func makeItem(
    eventType: String,
    decision: String?
  ) -> NotificationItem {
    NotificationItem(
      applicationName: "App",
      applicationAddress: "Address",
      applicationDescription: "Description",
      applicationType: "Type",
      authorityId: 1,
      createdAt: Date(timeIntervalSince1970: 1_712_000_000),
      eventType: eventType,
      decision: decision,
      sources: "Zone"
    )
  }

  // MARK: - Visibility

  @Test("renders UK display label for DecisionUpdate with Permitted")
  func decisionUpdate_permitted_returnsApproved() {
    let item = makeItem(eventType: "DecisionUpdate", decision: "Permitted")

    let label = NotificationDecisionBadge.displayLabel(for: item)

    #expect(label == "Approved")
  }

  @Test("renders UK display label for DecisionUpdate with Conditions")
  func decisionUpdate_conditions_returnsApprovedWithConditions() {
    let item = makeItem(eventType: "DecisionUpdate", decision: "Conditions")

    let label = NotificationDecisionBadge.displayLabel(for: item)

    #expect(label == "Approved with conditions")
  }

  @Test("renders UK display label for DecisionUpdate with Rejected")
  func decisionUpdate_rejected_returnsRefused() {
    let item = makeItem(eventType: "DecisionUpdate", decision: "Rejected")

    let label = NotificationDecisionBadge.displayLabel(for: item)

    #expect(label == "Refused")
  }

  @Test("renders UK display label for DecisionUpdate with Appealed")
  func decisionUpdate_appealed_returnsRefusalAppealed() {
    let item = makeItem(eventType: "DecisionUpdate", decision: "Appealed")

    let label = NotificationDecisionBadge.displayLabel(for: item)

    #expect(label == "Refusal appealed")
  }

  // MARK: - Suppression

  @Test("returns nil for NewApplication event type")
  func newApplication_returnsNil() {
    let item = makeItem(eventType: "NewApplication", decision: nil)

    let label = NotificationDecisionBadge.displayLabel(for: item)

    #expect(label == nil)
  }

  @Test("returns nil when DecisionUpdate has unrecognised decision vocab")
  func decisionUpdate_unknownVocab_returnsNil() {
    let item = makeItem(eventType: "DecisionUpdate", decision: "SomethingWeird")

    let label = NotificationDecisionBadge.displayLabel(for: item)

    #expect(label == nil)
  }

  @Test("returns nil when DecisionUpdate has nil decision")
  func decisionUpdate_nilDecision_returnsNil() {
    let item = makeItem(eventType: "DecisionUpdate", decision: nil)

    let label = NotificationDecisionBadge.displayLabel(for: item)

    #expect(label == nil)
  }

  @Test("returns nil when DecisionUpdate has empty decision string")
  func decisionUpdate_emptyDecision_returnsNil() {
    let item = makeItem(eventType: "DecisionUpdate", decision: "")

    let label = NotificationDecisionBadge.displayLabel(for: item)

    #expect(label == nil)
  }

  @Test("returns nil for unknown event type even with valid decision")
  func unknownEventType_validDecision_returnsNil() {
    let item = makeItem(eventType: "WeeklyDigest", decision: "Permitted")

    let label = NotificationDecisionBadge.displayLabel(for: item)

    #expect(label == nil)
  }
}
