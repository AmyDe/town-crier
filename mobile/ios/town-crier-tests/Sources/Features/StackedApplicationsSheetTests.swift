import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Smoke-tests ``StackedApplicationsSheet``'s presentation-only, closure-injected
/// API (GH#877) — it must build and render with no concrete `MapViewModel`
/// dependency, so both the authenticated and anonymous maps can present the
/// same sheet.
@MainActor
@Suite("StackedApplicationsSheet")
struct StackedApplicationsSheetTests {
  @Test func body_renders_withClosureInjectedOnSelect() {
    let stacked = StackedApplications(id: "stack-1", applications: [.pendingReview, .permitted])
    let sut = StackedApplicationsSheet(stacked: stacked) { _ in }

    _ = sut.body
  }

  @Test func onSelect_invokedWithTappedApplication() {
    let stacked = StackedApplications(id: "stack-1", applications: [.pendingReview, .permitted])
    var received: PlanningApplication?
    let sut = StackedApplicationsSheet(stacked: stacked) { received = $0 }

    sut.onSelect(.permitted)

    #expect(received == .permitted)
  }
}
