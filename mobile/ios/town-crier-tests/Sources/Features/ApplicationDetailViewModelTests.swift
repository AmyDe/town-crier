import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ApplicationDetailViewModel")
@MainActor
struct ApplicationDetailViewModelTests {

  // MARK: - Display Properties

  @Test func init_exposesApplicationDescription() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    #expect(sut.description == "Erection of two-storey rear extension")
  }

  @Test func init_exposesApplicationAddress() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    #expect(sut.address == "12 Mill Road, Cambridge, CB1 2AD")
  }

  @Test func init_exposesApplicationReference() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    #expect(sut.reference == "2026/0042")
  }

  @Test func init_exposesAuthorityName() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    #expect(sut.authorityName == "Cambridge")
  }

  // MARK: - Date Formatting

  @Test func receivedDateFormatted_usesUKLocaleFormat() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    // 1_700_000_000 = 14 Nov 2023 in UTC
    #expect(sut.receivedDateFormatted == "14 Nov 2023")
  }

  // MARK: - Status Display

  @Test func statusLabel_underReview_returnsPending() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    #expect(sut.statusLabel == "Pending")
  }

  @Test func statusLabel_approved_returnsApproved() {
    let sut = ApplicationDetailViewModel(application: .approved)

    #expect(sut.statusLabel == "Approved")
  }

  @Test func statusLabel_refused_returnsRefused() {
    let sut = ApplicationDetailViewModel(application: .refused)

    #expect(sut.statusLabel == "Refused")
  }

  @Test func statusLabel_withdrawn_returnsWithdrawn() {
    let sut = ApplicationDetailViewModel(application: .withdrawn)

    #expect(sut.statusLabel == "Withdrawn")
  }

  @Test func statusIcon_underReview_returnsClock() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    #expect(sut.statusIcon == "clock")
  }

  @Test func statusIcon_approved_returnsCheckmark() {
    let sut = ApplicationDetailViewModel(application: .approved)

    #expect(sut.statusIcon == "checkmark.circle")
  }

  @Test func statusIcon_refused_returnsXmark() {
    let sut = ApplicationDetailViewModel(application: .refused)

    #expect(sut.statusIcon == "xmark.circle")
  }

  // MARK: - Portal URL

  @Test func portalUrl_whenApplicationHasUrl_exposesIt() {
    let app = PlanningApplication.withPortalUrl
    let sut = ApplicationDetailViewModel(application: app)

    #expect(sut.portalUrl == URL(string: "https://planning.cambridge.gov.uk/2026/0042"))
  }

  @Test func portalUrl_whenApplicationHasNoUrl_returnsNil() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    #expect(sut.portalUrl == nil)
  }

  @Test func hasPortalUrl_whenUrlPresent_returnsTrue() {
    let sut = ApplicationDetailViewModel(application: .withPortalUrl)

    #expect(sut.hasPortalUrl)
  }

  @Test func hasPortalUrl_whenUrlAbsent_returnsFalse() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    #expect(!sut.hasPortalUrl)
  }

  // MARK: - Callbacks

  @Test func openPortal_invokesCallback() {
    let sut = ApplicationDetailViewModel(application: .withPortalUrl)
    var callbackUrl: URL?
    sut.onOpenPortal = { url in callbackUrl = url }

    sut.openPortal()

    #expect(callbackUrl == URL(string: "https://planning.cambridge.gov.uk/2026/0042"))
  }

  @Test func dismiss_invokesCallback() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)
    var dismissed = false
    sut.onDismiss = { dismissed = true }

    sut.dismiss()

    #expect(dismissed)
  }
}
