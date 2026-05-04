import Foundation
import Testing
import TownCrierDomain

/// Tests for ``LatestUnreadEvent`` — the per-application unread descriptor
/// surfaced on each row of the applications-by-zone result, used to drive
/// saturated/muted styling of ``ApplicationStatusPill`` on the Applications
/// screen and the `recent-activity` sort.
///
/// Spec: `docs/specs/notifications-unread-watermark.md#api-augment-applications`.
@Suite("LatestUnreadEvent")
struct LatestUnreadEventTests {

  @Test("stores type, decision, and createdAt verbatim")
  func init_storesFields() {
    let createdAt = Date(timeIntervalSince1970: 1_712_000_000)
    let event = LatestUnreadEvent(
      type: "DecisionUpdate",
      decision: "Permitted",
      createdAt: createdAt
    )

    #expect(event.type == "DecisionUpdate")
    #expect(event.decision == "Permitted")
    #expect(event.createdAt == createdAt)
  }

  @Test("decision is optional and may be nil for non-decision events")
  func init_allowsNilDecision() {
    let event = LatestUnreadEvent(
      type: "NewApplication",
      decision: nil,
      createdAt: Date(timeIntervalSince1970: 1_712_000_000)
    )

    #expect(event.decision == nil)
  }

  @Test("equatable when all fields match")
  func equatable_matchesOnAllFields() {
    let date = Date(timeIntervalSince1970: 1_712_000_000)
    let a = LatestUnreadEvent(type: "NewApplication", decision: nil, createdAt: date)
    let b = LatestUnreadEvent(type: "NewApplication", decision: nil, createdAt: date)

    #expect(a == b)
  }

  @Test("equatable distinguishes by createdAt")
  func equatable_distinguishesByCreatedAt() {
    let a = LatestUnreadEvent(
      type: "NewApplication",
      decision: nil,
      createdAt: Date(timeIntervalSince1970: 1_712_000_000)
    )
    let b = LatestUnreadEvent(
      type: "NewApplication",
      decision: nil,
      createdAt: Date(timeIntervalSince1970: 1_712_000_001)
    )

    #expect(a != b)
  }
}
