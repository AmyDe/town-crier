import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests for ``ApplicationListRow`` — the per-row presenter on the
/// Applications screen. The row delegates status rendering to
/// ``ApplicationStatusPill`` and toggles its `isMuted` knob based on whether
/// the row has a `latestUnreadEvent` (tc-1nsa.8). This keeps the row's
/// muted/saturated semantics aligned with the web counterpart from
/// tc-1nsa.11.
@Suite("ApplicationListRow")
@MainActor
struct ApplicationListRowTests {

  // MARK: - Saturation

  @Test("renders saturated pill when latestUnreadEvent is non-nil")
  func unread_pillIsSaturated() {
    let unread = PlanningApplication.permitted.withLatestUnreadEvent(
      LatestUnreadEvent(
        type: "DecisionUpdate",
        decision: "Permitted",
        createdAt: Date(timeIntervalSince1970: 1_700_000_500)
      )
    )

    let sut = ApplicationListRow(application: unread)

    #expect(!sut.statusPill.isMuted)
  }

  @Test("renders muted pill when latestUnreadEvent is nil")
  func read_pillIsMuted() {
    let read = PlanningApplication.permitted.withLatestUnreadEvent(nil)

    let sut = ApplicationListRow(application: read)

    #expect(sut.statusPill.isMuted)
  }

  // MARK: - Vocabulary delegation

  @Test("pill surfaces the application's status verbatim")
  func pill_carriesApplicationStatus() {
    let app = PlanningApplication.rejected
    let sut = ApplicationListRow(application: app)

    #expect(sut.statusPill.status == app.status)
  }
}
