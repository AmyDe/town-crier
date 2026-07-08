import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@MainActor
@Suite("AnonymousApplicationSummaryView")
struct AnonymousApplicationSummaryViewTests {
  @Test func body_renders() {
    let sut = AnonymousApplicationSummaryView(application: .pendingReview) {}
    _ = sut.body
  }

  @Test func onSignUp_invokedByCaller() {
    var invoked = false
    let sut = AnonymousApplicationSummaryView(application: .pendingReview) { invoked = true }
    sut.onSignUp()
    #expect(invoked)
  }
}
