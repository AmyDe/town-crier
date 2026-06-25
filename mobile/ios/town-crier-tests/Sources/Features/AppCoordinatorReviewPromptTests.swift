import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Integration tests for how ``AppCoordinator`` and its extensions feed the
/// review-prompt tracker (GH #628): the upgrade signal is latched to a free→paid
/// transition, friction/onboarding/post-purchase moments suppress the session,
/// and the value moments (deep-link alert open, portal tap, save) record their
/// signals.
@Suite("AppCoordinator — review prompt wiring")
@MainActor
struct AppCoordinatorReviewPromptTests {
  private let reference = Date(timeIntervalSince1970: 1_700_000_000)
  private let day: TimeInterval = 86_400

  private func eligibleState(
    engagementScore: Int = 0,
    saveCount: Int = 0
  ) -> ReviewPromptState {
    ReviewPromptState(
      firstLaunchDate: reference.addingTimeInterval(-30 * day),
      engagementScore: engagementScore,
      saveCount: saveCount
    )
  }

  private func makeSUT(
    storeState: ReviewPromptState,
    resolvedTier: SubscriptionTier = .personal,
    authorizationStatus: NotificationAuthorizationStatus = .authorized,
    savedApplicationRepository: SavedApplicationRepository? = nil,
    watchZones: [WatchZone] = []
  ) -> (AppCoordinator, FakeReviewPromptStore, SpyReviewRequester) {
    let store = FakeReviewPromptStore(state: storeState)
    let requester = SpyReviewRequester()
    let now = reference
    let tracker = ReviewPromptTracker(store: store, requester: requester) { now }

    let notificationSpy = SpyNotificationService()
    notificationSpy.nextAuthorizationStatus = authorizationStatus
    let tierResolver = FakeSubscriptionTierResolver()
    tierResolver.resolveResult = (resolvedTier, false)
    let watchZoneRepository = SpyWatchZoneRepository()
    watchZoneRepository.loadAllResult = .success(watchZones)

    let coordinator = AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      tierResolver: tierResolver,
      watchZoneRepository: watchZoneRepository,
      geocoder: SpyPostcodeGeocoder(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: notificationSpy,
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      savedApplicationRepository: savedApplicationRepository,
      tierCache: UserDefaults(suiteName: UUID().uuidString) ?? .standard,
      reviewPromptTracker: tracker
    )
    return (coordinator, store, requester)
  }

  // MARK: - Upgrade

  @Test("a free→paid transition records the upgrade once and never fires")
  func upgradeRecordedOnceOnTransition() async {
    let (sut, store, requester) = makeSUT(storeState: eligibleState(engagementScore: 0))

    await sut.resolveSubscriptionTier()
    #expect(store.load().engagementScore == 2)
    #expect(store.load().hasRecordedUpgrade)

    // A second resolve (now paid→paid) must not re-record the upgrade.
    await sut.resolveSubscriptionTier()
    #expect(store.load().engagementScore == 2)
    #expect(requester.requestReviewCallCount == 0)
  }

  // MARK: - Suppression

  @Test("the post-purchase push prompt suppresses the review session")
  func postPurchasePushPromptSuppresses() async {
    let (sut, _, requester) = makeSUT(
      storeState: eligibleState(engagementScore: 4),
      authorizationStatus: .notDetermined
    )

    await sut.resolveSubscriptionTier()
    await sut.waitForPendingPostPurchasePrompt()
    sut.reviewPromptTracker?.record(.openedAlert)  // would reach the threshold

    #expect(requester.requestReviewCallCount == 0)
  }

  @Test("without the post-purchase prompt a value moment can still fire")
  func noPushPromptAllowsFire() async {
    let (sut, _, requester) = makeSUT(
      storeState: eligibleState(engagementScore: 4),
      authorizationStatus: .authorized
    )

    await sut.resolveSubscriptionTier()
    sut.reviewPromptTracker?.record(.openedAlert)

    #expect(requester.requestReviewCallCount == 1)
  }

  @Test("an onboarding-required session is suppressed")
  func onboardingRequiredSuppresses() async {
    let (sut, _, requester) = makeSUT(
      storeState: eligibleState(engagementScore: 4),
      watchZones: []
    )

    await sut.determineOnboarding()
    sut.reviewPromptTracker?.record(.openedAlert)

    #expect(requester.requestReviewCallCount == 0)
  }

  @Test("a returning user with zones is not suppressed by onboarding")
  func onboardingNotRequiredDoesNotSuppress() async {
    let (sut, _, requester) = makeSUT(
      storeState: eligibleState(engagementScore: 4),
      watchZones: [.cambridge]
    )

    await sut.determineOnboarding()
    sut.reviewPromptTracker?.record(.openedAlert)

    #expect(requester.requestReviewCallCount == 1)
  }

  // MARK: - Value moments

  @Test("opening an application via a deep link records the alert signal")
  func deepLinkRecordsOpenedAlert() async {
    let (sut, _, requester) = makeSUT(storeState: eligibleState(engagementScore: 4))

    sut.handleDeepLink(.applicationDetail(PlanningApplicationId(authority: "CAM", name: "2026/0042")))
    await sut.waitForPendingDetailLoad()

    #expect(requester.requestReviewCallCount == 1)
  }

  @Test("tapping through to the portal records the portal signal")
  func portalTapRecordsTappedPortal() {
    let (sut, _, requester) = makeSUT(storeState: eligibleState(engagementScore: 3))

    let viewModel = sut.makeApplicationDetailViewModel(application: .withPortalUrl)
    viewModel.openPortal()

    #expect(requester.requestReviewCallCount == 1)
  }

  @Test("a successful save records the saved-application signal")
  func saveRecordsSavedApplication() async {
    let (sut, _, requester) = makeSUT(
      storeState: eligibleState(engagementScore: 4, saveCount: 1),
      savedApplicationRepository: SpySavedApplicationRepository()
    )

    let viewModel = sut.makeApplicationDetailViewModel(application: .withPortalUrl)
    await viewModel.toggleSave()  // 2nd save -> +2 -> 6

    #expect(requester.requestReviewCallCount == 1)
  }
}
