import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests for ``ApplicationListRow`` — the per-row presenter on the
/// Applications screen. The row signals unread state via a leading-aligned
/// accent dot when `latestUnreadEvent != nil`; rows with no unread event
/// render a same-size transparent placeholder so columns stay aligned. This
/// keeps the iOS row visually aligned with the web `ApplicationCard` from
/// tc-1nsa.11 (spec decision #6) and isolates the muted/saturated affordance
/// from ``ApplicationStatusPill``.
@Suite("ApplicationListRow")
@MainActor
struct ApplicationListRowTests {

  // MARK: - Unread dot

  @Test("renders unread dot when latestUnreadEvent is non-nil")
  func unread_dotIsVisible() {
    let unread = PlanningApplication.permitted.withLatestUnreadEvent(
      LatestUnreadEvent(
        type: "DecisionUpdate",
        decision: "Permitted",
        createdAt: Date(timeIntervalSince1970: 1_700_000_500)
      )
    )

    let sut = ApplicationListRow(application: unread)

    #expect(sut.hasUnreadDot)
  }

  @Test("hides unread dot (placeholder only) when latestUnreadEvent is nil")
  func read_dotIsHidden() {
    let read = PlanningApplication.permitted.withLatestUnreadEvent(nil)

    let sut = ApplicationListRow(application: read)

    #expect(!sut.hasUnreadDot)
  }

  // MARK: - Vocabulary delegation

  @Test("pill surfaces the application's status verbatim")
  func pill_carriesApplicationStatus() {
    let app = PlanningApplication.rejected
    let sut = ApplicationListRow(application: app)

    #expect(sut.statusPill.status == app.status)
  }
}
