import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Post-purchase notification prompt (issue #624, Prong 1).
///
/// After a purchase resolves to a paid tier and notifications have never been
/// requested (`.notDetermined`), `resolveSubscriptionTier()` triggers the
/// system permission prompt — the highest-intent moment to ask. Gating on
/// `.notDetermined` self-limits the prompt to once: a `.denied` user is never
/// re-prompted (iOS won't re-show the dialog anyway), and a `.free` user is
/// never prompted regardless of status. The prompt is fired into a stored
/// `Task` so tests can await it deterministically.
@Suite("AppCoordinator — post-purchase notification prompt")
@MainActor
struct AppCoordinatorPostPurchaseNotificationTests {

  private func makeSUT(
    resolvedTier: SubscriptionTier,
    authorizationStatus: NotificationAuthorizationStatus
  ) -> (AppCoordinator, SpyNotificationService) {
    let notificationSpy = SpyNotificationService()
    notificationSpy.nextAuthorizationStatus = authorizationStatus
    let tierResolver = FakeSubscriptionTierResolver()
    tierResolver.resolveResult = (resolvedTier, false)
    let coordinator = AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      tierResolver: tierResolver,
      watchZoneRepository: SpyWatchZoneRepository(),
      geocoder: SpyPostcodeGeocoder(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: notificationSpy,
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      tierCache: UserDefaults(suiteName: UUID().uuidString) ?? .standard
    )
    return (coordinator, notificationSpy)
  }

  @Test func paidTier_notDetermined_requestsPermission() async {
    let (sut, notificationSpy) = makeSUT(
      resolvedTier: .personal,
      authorizationStatus: .notDetermined
    )

    await sut.resolveSubscriptionTier()
    await sut.waitForPendingPostPurchasePrompt()

    #expect(notificationSpy.requestPermissionCallCount == 1)
  }

  @Test func proTier_notDetermined_requestsPermission() async {
    let (sut, notificationSpy) = makeSUT(
      resolvedTier: .pro,
      authorizationStatus: .notDetermined
    )

    await sut.resolveSubscriptionTier()
    await sut.waitForPendingPostPurchasePrompt()

    #expect(notificationSpy.requestPermissionCallCount == 1)
  }

  @Test func paidTier_authorized_doesNotRequest() async {
    let (sut, notificationSpy) = makeSUT(
      resolvedTier: .personal,
      authorizationStatus: .authorized
    )

    await sut.resolveSubscriptionTier()
    await sut.waitForPendingPostPurchasePrompt()

    #expect(notificationSpy.requestPermissionCallCount == 0)
  }

  @Test func paidTier_denied_doesNotRequest() async {
    let (sut, notificationSpy) = makeSUT(
      resolvedTier: .personal,
      authorizationStatus: .denied
    )

    await sut.resolveSubscriptionTier()
    await sut.waitForPendingPostPurchasePrompt()

    #expect(notificationSpy.requestPermissionCallCount == 0)
  }

  @Test func freeTier_notDetermined_doesNotRequest() async {
    let (sut, notificationSpy) = makeSUT(
      resolvedTier: .free,
      authorizationStatus: .notDetermined
    )

    await sut.resolveSubscriptionTier()
    await sut.waitForPendingPostPurchasePrompt()

    #expect(notificationSpy.requestPermissionCallCount == 0)
  }

  // MARK: - Banner factory wiring

  @Test func makePushNudgeViewModel_usesResolvedTier_andDeniedActionOpensSettings() async {
    let (sut, notificationSpy) = makeSUT(
      resolvedTier: .personal,
      authorizationStatus: .denied
    )
    await sut.resolveSubscriptionTier()

    let viewModel = sut.makePushNudgeViewModel()
    await viewModel.load()
    #expect(viewModel.isVisible == true)

    await viewModel.primaryAction()

    #expect(sut.isOpeningSystemNotificationSettings == true)
    #expect(notificationSpy.requestPermissionCallCount == 0)
  }
}
