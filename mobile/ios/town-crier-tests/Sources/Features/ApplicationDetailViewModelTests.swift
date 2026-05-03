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

  @Test func statusLabel_undecided_returnsPending() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    #expect(sut.statusLabel == "Pending")
  }

  @Test func statusLabel_permitted_returnsGranted() {
    let sut = ApplicationDetailViewModel(application: .permitted)

    #expect(sut.statusLabel == "Granted")
  }

  @Test func statusLabel_rejected_returnsRefused() {
    let sut = ApplicationDetailViewModel(application: .rejected)

    #expect(sut.statusLabel == "Refused")
  }

  @Test func statusLabel_withdrawn_returnsWithdrawn() {
    let sut = ApplicationDetailViewModel(application: .withdrawn)

    #expect(sut.statusLabel == "Withdrawn")
  }

  @Test func statusIcon_undecided_returnsClock() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    #expect(sut.statusIcon == "clock")
  }

  @Test func statusIcon_permitted_returnsCheckmark() {
    let sut = ApplicationDetailViewModel(application: .permitted)

    #expect(sut.statusIcon == "checkmark.circle")
  }

  @Test func statusIcon_rejected_returnsXmark() {
    let sut = ApplicationDetailViewModel(application: .rejected)

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

  // MARK: - Stale-While-Revalidate refresh (tc-sslz)

  @Test func refresh_updatesApplication_whenServerReturnsNewerPayload() async {
    let repo = SpyPlanningApplicationRepository()
    repo.fetchApplicationResult = .success(.permitted)
    let sut = ApplicationDetailViewModel(
      application: .pendingReview,
      planningApplicationRepository: repo
    )

    await sut.refresh()

    #expect(sut.statusLabel == "Granted")
    #expect(sut.reference == "2026/0099")
  }

  @Test func refresh_callsRepositoryWithApplicationId() async {
    let repo = SpyPlanningApplicationRepository()
    repo.fetchApplicationResult = .success(.pendingReview)
    let sut = ApplicationDetailViewModel(
      application: .pendingReview,
      planningApplicationRepository: repo
    )

    await sut.refresh()

    #expect(repo.fetchApplicationCalls == [PlanningApplication.pendingReview.id])
  }

  @Test func refresh_keepsCachedPayload_whenRepositoryFails() async {
    let repo = SpyPlanningApplicationRepository()
    repo.fetchApplicationResult = .failure(DomainError.networkUnavailable)
    let sut = ApplicationDetailViewModel(
      application: .pendingReview,
      planningApplicationRepository: repo
    )

    await sut.refresh()

    // Cached payload remains visible — refresh is silent on error.
    #expect(sut.reference == "2026/0042")
    #expect(sut.statusLabel == "Pending")
  }

  @Test func refresh_isNoOp_whenRepositoryNotInjected() async {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    await sut.refresh()

    // No crash and the cached payload is unchanged.
    #expect(sut.reference == "2026/0042")
  }
}
