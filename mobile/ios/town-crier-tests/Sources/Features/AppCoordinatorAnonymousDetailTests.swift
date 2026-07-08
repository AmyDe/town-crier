import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#879 Phase 2: a signed-out tap on a share Universal Link, or the
/// anonymous map/summary sheet's "View full details", must present the
/// detail sheet without ever surfacing the authed repository's
/// `sessionExpired` error, and the resulting view model must be configured
/// for anonymous mode (no Save, sign-up CTA, by-slug refresh).
@Suite("AppCoordinator — anonymous detail (GH#879 Phase 2)")
@MainActor
struct AppCoordinatorAnonymousDetailTests {
  private func makeSUT(
    hasSession: Bool,
    savedApplicationRepository: SavedApplicationRepository? = nil
  ) -> (
    AppCoordinator, SpyPlanningApplicationRepository, SpyAnonymousApplicationDetailRepository,
    SpyAuthenticationService
  ) {
    let planningSpy = SpyPlanningApplicationRepository()
    let anonSpy = SpyAnonymousApplicationDetailRepository()
    let authSpy = SpyAuthenticationService()
    authSpy.currentSessionResult = hasSession ? .valid : nil
    let coordinator = AppCoordinator(
      repository: planningSpy,
      authService: authSpy,
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      savedApplicationRepository: savedApplicationRepository,
      anonymousApplicationDetailRepository: anonSpy
    )
    return (coordinator, planningSpy, anonSpy, authSpy)
  }

  /// Carries an authority slug, unlike `.permitted`/`.rejected` — required for
  /// exercising `refresh()`'s by-slug read.
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

  // MARK: - Share link routing

  @Test func showApplicationDetailBySlug_noSession_usesAnonymousRepository() async {
    let (sut, planningSpy, anonSpy, _) = makeSUT(hasSession: false)
    anonSpy.fetchApplicationBySlugResult = .success(.permitted)

    sut.showApplicationDetail(bySlug: "kingston", ref: "Kingston/25/02755/CLC")
    await sut.waitForPendingDetailLoad()

    #expect(sut.detailApplication == .permitted)
    #expect(
      anonSpy.fetchApplicationBySlugCalls == [
        .init(authoritySlug: "kingston", ref: "Kingston/25/02755/CLC")
      ])
    #expect(planningSpy.fetchApplicationBySlugCalls.isEmpty)
    // The bug this fixes: no session must never surface sessionExpired.
    #expect(sut.deepLinkError == nil)
  }

  @Test func showApplicationDetailBySlug_withSession_usesAuthedRepository() async {
    let (sut, planningSpy, anonSpy, _) = makeSUT(hasSession: true)
    planningSpy.fetchApplicationBySlugResult = .success(.permitted)

    sut.showApplicationDetail(bySlug: "kingston", ref: "Kingston/25/02755/CLC")
    await sut.waitForPendingDetailLoad()

    #expect(sut.detailApplication == .permitted)
    #expect(
      planningSpy.fetchApplicationBySlugCalls == [
        .init(authoritySlug: "kingston", ref: "Kingston/25/02755/CLC")
      ])
    #expect(anonSpy.fetchApplicationBySlugCalls.isEmpty)
  }

  @Test func showApplicationDetailBySlug_noSessionNoAnonymousRepositoryInjected_fallsBackToAuthedRepository()
    async {
    // Defensive fallback for a coordinator built without the anonymous
    // repository (e.g. an older/incomplete composition root) — preserves
    // today's behaviour rather than silently doing nothing.
    let planningSpy = SpyPlanningApplicationRepository()
    planningSpy.fetchApplicationBySlugResult = .success(.permitted)
    let authSpy = SpyAuthenticationService()
    authSpy.currentSessionResult = nil
    let sut = AppCoordinator(
      repository: planningSpy,
      authService: authSpy,
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )

    sut.showApplicationDetail(bySlug: "kingston", ref: "Kingston/25/02755/CLC")
    await sut.waitForPendingDetailLoad()

    #expect(sut.detailApplication == .permitted)
    #expect(planningSpy.fetchApplicationBySlugCalls.count == 1)
  }

  // MARK: - makeApplicationDetailViewModel configuration

  @Test func makeApplicationDetailViewModel_afterAnonymousShareLink_hidesSaveAndShowsSignUpCTA() async {
    let (sut, _, anonSpy, _) = makeSUT(hasSession: false)
    anonSpy.fetchApplicationBySlugResult = .success(.permitted)
    sut.showApplicationDetail(bySlug: "kingston", ref: "Kingston/25/02755/CLC")
    await sut.waitForPendingDetailLoad()

    let vm = sut.makeApplicationDetailViewModel(application: .permitted)

    #expect(!vm.canSave)
    #expect(vm.showsSignUpCTA)
  }

  @Test func makeApplicationDetailViewModel_afterAnonymousShareLink_refreshUsesAnonymousRepository() async {
    let (sut, _, anonSpy, _) = makeSUT(hasSession: false)
    // `kingstonApplication` carries an authority slug — required for
    // refresh()'s by-slug read; the other fixtures (.permitted, etc.) don't.
    anonSpy.fetchApplicationBySlugResult = .success(kingstonApplication)
    sut.showApplicationDetail(bySlug: "kingston", ref: "Kingston/25/02755/CLC")
    await sut.waitForPendingDetailLoad()
    anonSpy.fetchApplicationBySlugResult = .success(.rejected)

    let vm = sut.makeApplicationDetailViewModel(application: kingstonApplication)
    await vm.refresh()

    #expect(anonSpy.fetchApplicationBySlugCalls.count == 2)
  }

  @Test func makeApplicationDetailViewModel_afterAuthedShareLink_keepsAuthedBehaviour() async {
    let savedSpy = SpySavedApplicationRepository()
    let (sut, planningSpy, _, _) = makeSUT(hasSession: true, savedApplicationRepository: savedSpy)
    planningSpy.fetchApplicationBySlugResult = .success(.permitted)
    sut.showApplicationDetail(bySlug: "kingston", ref: "Kingston/25/02755/CLC")
    await sut.waitForPendingDetailLoad()

    let vm = sut.makeApplicationDetailViewModel(application: .permitted)

    #expect(vm.canSave)
    #expect(!vm.showsSignUpCTA)
  }

  // MARK: - showAnonymousApplicationDetail (in-app "View full details")

  @Test func showAnonymousApplicationDetail_setsDetailSynchronously() {
    let (sut, _, _, _) = makeSUT(hasSession: false)

    sut.showAnonymousApplicationDetail(.permitted)

    #expect(sut.detailApplication == .permitted)
  }

  @Test func makeApplicationDetailViewModel_afterShowAnonymousApplicationDetail_hidesSaveAndShowsSignUpCTA() {
    let (sut, _, _, _) = makeSUT(hasSession: false)
    sut.showAnonymousApplicationDetail(.permitted)

    let vm = sut.makeApplicationDetailViewModel(application: .permitted)

    #expect(!vm.canSave)
    #expect(vm.showsSignUpCTA)
  }

  @Test func makeApplicationDetailViewModel_afterAuthedShowApplicationDetail_neverShowsSignUpCTA() {
    // Regression guard: a stale anonymous flag from a prior anonymous
    // session must never leak into an authed detail open.
    let savedSpy = SpySavedApplicationRepository()
    let (sut, _, _, _) = makeSUT(hasSession: true, savedApplicationRepository: savedSpy)
    sut.showAnonymousApplicationDetail(.pendingReview)

    sut.showApplicationDetail(.permitted)
    let vm = sut.makeApplicationDetailViewModel(application: .permitted)

    #expect(vm.canSave)
    #expect(!vm.showsSignUpCTA)
  }

  // MARK: - Sign-up CTA callback wiring

  @Test func makeApplicationDetailViewModel_requestSignUp_invokesCoordinatorOnRequestSignUp() {
    let (sut, _, _, _) = makeSUT(hasSession: false)
    var invoked = false
    sut.onRequestSignUp = { invoked = true }
    sut.showAnonymousApplicationDetail(.permitted)
    let vm = sut.makeApplicationDetailViewModel(application: .permitted)

    vm.requestSignUp()

    #expect(invoked)
  }
}
