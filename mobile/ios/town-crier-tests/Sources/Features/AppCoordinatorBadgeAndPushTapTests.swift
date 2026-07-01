import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Foreground badge sync + push-tap per-application mark-read (tc-0sfx.3).
///
/// Tapping a push about an application means the user has seen it, so the
/// deep-linked application is marked read (ADR 0035). A push with no
/// application deep link (e.g. a digest push) marks nothing.
@Suite("AppCoordinator — badge sync & push-tap mark-read")
@MainActor
struct AppCoordinatorBadgeAndPushTapTests {
  private func makeSUT(
    notificationStateRepository: NotificationStateRepository? = nil,
    badgeSetter: BadgeSetting? = nil
  ) -> AppCoordinator {
    AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      notificationStateRepository: notificationStateRepository,
      badgeSetter: badgeSetter
    )
  }

  private func makeSUT(
    planningRepository: SpyPlanningApplicationRepository,
    notificationStateRepository: NotificationStateRepository
  ) -> AppCoordinator {
    AppCoordinator(
      repository: planningRepository,
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      notificationStateRepository: notificationStateRepository
    )
  }

  // MARK: - syncBadge

  @Test func syncBadge_setsBadgeToTotalUnreadCount() async {
    let stateRepo = SpyNotificationStateRepository()
    stateRepo.fetchStateResult = .success(
      NotificationState(
        lastReadAt: Date(timeIntervalSince1970: 0),
        version: 1,
        totalUnreadCount: 7
      )
    )
    let badgeSetter = SpyBadgeSetter()
    let sut = makeSUT(
      notificationStateRepository: stateRepo,
      badgeSetter: badgeSetter
    )

    await sut.syncBadge()

    #expect(stateRepo.fetchStateCallCount == 1)
    #expect(badgeSetter.setBadgeCalls == [7])
  }

  @Test func syncBadge_withZeroUnread_clearsBadge() async {
    let stateRepo = SpyNotificationStateRepository()
    stateRepo.fetchStateResult = .success(
      NotificationState(
        lastReadAt: Date(timeIntervalSince1970: 0),
        version: 1,
        totalUnreadCount: 0
      )
    )
    let badgeSetter = SpyBadgeSetter()
    let sut = makeSUT(
      notificationStateRepository: stateRepo,
      badgeSetter: badgeSetter
    )

    await sut.syncBadge()

    #expect(badgeSetter.setBadgeCalls == [0])
  }

  @Test func syncBadge_swallowsFetchErrors() async {
    // Foreground sync is best-effort. A fetch failure must not crash; the
    // badge stays at whatever the OS already shows.
    let stateRepo = SpyNotificationStateRepository()
    stateRepo.fetchStateResult = .failure(DomainError.networkUnavailable)
    let badgeSetter = SpyBadgeSetter()
    let sut = makeSUT(
      notificationStateRepository: stateRepo,
      badgeSetter: badgeSetter
    )

    await sut.syncBadge()

    #expect(badgeSetter.setBadgeCalls.isEmpty)
  }

  @Test func syncBadge_withoutRepository_isNoop() async {
    let badgeSetter = SpyBadgeSetter()
    let sut = makeSUT(
      notificationStateRepository: nil,
      badgeSetter: badgeSetter
    )

    await sut.syncBadge()

    #expect(badgeSetter.setBadgeCalls.isEmpty)
  }

  @Test func syncBadge_withoutBadgeSetter_doesNotCrash() async {
    let stateRepo = SpyNotificationStateRepository()
    stateRepo.fetchStateResult = .success(
      NotificationState(
        lastReadAt: Date(timeIntervalSince1970: 0),
        version: 1,
        totalUnreadCount: 3
      )
    )
    let sut = makeSUT(
      notificationStateRepository: stateRepo,
      badgeSetter: nil
    )

    await sut.syncBadge()

    #expect(stateRepo.fetchStateCallCount == 1)
  }

  // MARK: - handlePushTap (deep-link routing + per-application mark-read)

  @Test func handlePushTap_withApplicationDeepLink_routesAndMarksRead() async {
    // A push carrying an application deep link routes to the detail AND marks
    // that application read via the composite (applicationUid, authorityId).
    let stateRepo = SpyNotificationStateRepository()
    let planningSpy = SpyPlanningApplicationRepository()
    planningSpy.fetchApplicationResult = .success(.permitted)
    let sut = makeSUT(
      planningRepository: planningSpy,
      notificationStateRepository: stateRepo
    )
    let userInfo: [AnyHashable: Any] = [
      "applicationRef": "APP-002",
      "authorityId": 42,
    ]

    sut.handlePushTap(userInfo: userInfo)

    await sut.waitForPendingDetailLoad()
    await sut.waitForPendingApplicationMarkRead()

    #expect(
      planningSpy.fetchApplicationCalls == [PlanningApplicationId(authority: "42", name: "APP-002")]
    )
    #expect(
      stateRepo.markApplicationReadCalls == [
        MarkApplicationReadCall(applicationUid: "APP-002", authorityId: 42)
      ]
    )
  }

  @Test func handlePushTap_marksRead_swallowsRepositoryErrors() async {
    // Push-tap mark-read is fire-and-forget: a network blip must not propagate
    // to the user — the deep-link presentation has already happened.
    let stateRepo = SpyNotificationStateRepository()
    stateRepo.markApplicationReadResult = .failure(DomainError.networkUnavailable)
    let planningSpy = SpyPlanningApplicationRepository()
    planningSpy.fetchApplicationResult = .success(.permitted)
    let sut = makeSUT(
      planningRepository: planningSpy,
      notificationStateRepository: stateRepo
    )
    let userInfo: [AnyHashable: Any] = ["applicationRef": "APP-002", "authorityId": 42]

    sut.handlePushTap(userInfo: userInfo)

    await sut.waitForPendingDetailLoad()
    await sut.waitForPendingApplicationMarkRead()

    #expect(stateRepo.markApplicationReadCalls.count == 1)
  }

  @Test func handlePushTap_digestPayload_doesNothing() async {
    // Digest pushes contain no applicationRef. The coordinator must no-op
    // rather than fire spurious requests — there is no single app to mark.
    let stateRepo = SpyNotificationStateRepository()
    let planningSpy = SpyPlanningApplicationRepository()
    let sut = makeSUT(
      planningRepository: planningSpy,
      notificationStateRepository: stateRepo
    )
    let userInfo: [AnyHashable: Any] = [
      "aps": ["alert": ["title": "Town Crier", "body": "5 new"], "badge": 5]
    ]

    sut.handlePushTap(userInfo: userInfo)

    await sut.waitForPendingDetailLoad()
    await sut.waitForPendingApplicationMarkRead()

    #expect(planningSpy.fetchApplicationCalls.isEmpty)
    #expect(stateRepo.markApplicationReadCalls.isEmpty)
  }
}
