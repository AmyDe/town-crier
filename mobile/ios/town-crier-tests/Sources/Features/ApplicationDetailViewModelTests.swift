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

  // MARK: - Share URL (GH #738 Slice 4)

  @Test func shareURL_whenAuthoritySlugPresent_buildsCanonicalShareURL() {
    let application = PlanningApplication(
      id: PlanningApplicationId(authority: "789", name: "Kingston/25/02755/CLC"),
      reference: ApplicationReference("Kingston/25/02755/CLC"),
      authority: LocalAuthority(code: "789", name: "Kingston upon Thames", slug: "kingston"),
      status: .undecided,
      receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
      description: "Certificate of lawfulness",
      address: "1 Market Place, Kingston, KT1 1JS"
    )
    let sut = ApplicationDetailViewModel(application: application)

    #expect(
      sut.shareURL?.absoluteString
        == "https://share.towncrierapp.uk/a/kingston/Kingston/25/02755/CLC")
  }

  @Test func shareURL_whenAuthoritySlugAbsent_isNil() {
    // `.pendingReview` carries no authority slug (list-payload shape), so no
    // slug-less broken link is ever offered.
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    #expect(sut.shareURL == nil)
  }

  // MARK: - Anonymous mode (GH#879 Phase 2)

  private var kingstonApplication: PlanningApplication {
    PlanningApplication(
      id: PlanningApplicationId(authority: "789", name: "Kingston/25/02755/CLC"),
      reference: ApplicationReference("Kingston/25/02755/CLC"),
      authority: LocalAuthority(code: "789", name: "Kingston upon Thames", slug: "kingston"),
      status: .undecided,
      receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
      description: "Certificate of lawfulness",
      address: "1 Market Place, Kingston, KT1 1JS"
    )
  }

  @Test func showsSignUpCTA_isTrue_whenAnonymousRepositoryInjected() {
    let sut = ApplicationDetailViewModel(
      application: .pendingReview,
      anonymousApplicationDetailRepository: SpyAnonymousApplicationDetailRepository()
    )

    #expect(sut.showsSignUpCTA)
  }

  @Test func showsSignUpCTA_isFalse_byDefault() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    #expect(!sut.showsSignUpCTA)
  }

  @Test func canSave_isFalse_inAnonymousMode() {
    // Anonymous construction never injects a SavedApplicationRepository, so
    // Save stays hidden regardless of the anonymous repository being present.
    let sut = ApplicationDetailViewModel(
      application: .pendingReview,
      anonymousApplicationDetailRepository: SpyAnonymousApplicationDetailRepository()
    )

    #expect(!sut.canSave)
  }

  @Test func loadSavedState_isNoOp_inAnonymousMode() async {
    // No SavedApplicationRepository is injected in anonymous mode, so this
    // stays a no-op exactly like the existing "no repository" case.
    let sut = ApplicationDetailViewModel(
      application: .pendingReview,
      anonymousApplicationDetailRepository: SpyAnonymousApplicationDetailRepository()
    )

    await sut.loadSavedState()

    #expect(!sut.isSaved)
  }

  @Test func requestSignUp_invokesOnRequestSignUpCallback() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)
    var invoked = false
    sut.onRequestSignUp = { invoked = true }

    sut.requestSignUp()

    #expect(invoked)
  }

  @Test func refresh_inAnonymousMode_callsAnonymousRepositoryBySlug() async {
    let anonSpy = SpyAnonymousApplicationDetailRepository()
    anonSpy.fetchApplicationBySlugResult = .success(.permitted)
    let sut = ApplicationDetailViewModel(
      application: kingstonApplication,
      anonymousApplicationDetailRepository: anonSpy
    )

    await sut.refresh()

    #expect(
      anonSpy.fetchApplicationBySlugCalls == [
        .init(authoritySlug: "kingston", ref: "Kingston/25/02755/CLC")
      ])
    #expect(sut.reference == "2026/0099")
  }

  @Test func refresh_inAnonymousMode_skipsSilently_whenApplicationHasNoSlug() async {
    // `.pendingReview` carries no authority slug — refreshing it by-slug would
    // be meaningless, so the anonymous repository must never be called.
    let anonSpy = SpyAnonymousApplicationDetailRepository()
    let sut = ApplicationDetailViewModel(
      application: .pendingReview,
      anonymousApplicationDetailRepository: anonSpy
    )

    await sut.refresh()

    #expect(anonSpy.fetchApplicationBySlugCalls.isEmpty)
    #expect(sut.reference == "2026/0042")
  }

  @Test func refresh_inAnonymousMode_doesNotCallPlanningApplicationRepository() async {
    // Guards against a future regression where both repositories are
    // injected and the authed by-id path fires instead of the by-slug one.
    let planningSpy = SpyPlanningApplicationRepository()
    let anonSpy = SpyAnonymousApplicationDetailRepository()
    anonSpy.fetchApplicationBySlugResult = .success(.permitted)
    let sut = ApplicationDetailViewModel(
      application: kingstonApplication,
      planningApplicationRepository: planningSpy,
      anonymousApplicationDetailRepository: anonSpy
    )

    await sut.refresh()

    #expect(planningSpy.fetchApplicationCalls.isEmpty)
  }
}
