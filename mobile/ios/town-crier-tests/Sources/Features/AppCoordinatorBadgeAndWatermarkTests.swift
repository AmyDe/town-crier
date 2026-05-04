import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Foreground badge sync + push-tap watermark advance (tc-1nsa.9).
///
/// Spec: docs/specs/notifications-unread-watermark.md#ios-badge-foreground-push.
@Suite("AppCoordinator — badge sync & watermark advance")
@MainActor
struct AppCoordinatorBadgeAndWatermarkTests {
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

  // MARK: - advanceWatermark

  @Test func advanceWatermark_callsRepositoryWithSuppliedInstant() async {
    let stateRepo = SpyNotificationStateRepository()
    let asOf = Date(timeIntervalSince1970: 1_712_000_500)
    let sut = makeSUT(notificationStateRepository: stateRepo)

    sut.advanceWatermark(asOf: asOf)
    await sut.waitForPendingWatermarkAdvance()

    #expect(stateRepo.advanceCalls == [asOf])
  }

  @Test func advanceWatermark_withoutRepository_isNoop() async {
    // The Coordinator may be constructed without a NotificationStateRepository
    // (legacy/test wiring); calling advanceWatermark must not crash.
    let sut = makeSUT(notificationStateRepository: nil)

    sut.advanceWatermark(asOf: Date())
    await sut.waitForPendingWatermarkAdvance()
  }

  @Test func advanceWatermark_swallowsRepositoryErrors() async {
    // Push-tap is fire-and-forget. A network blip must not propagate to the
    // user — the deep-link presentation has already happened.
    let stateRepo = SpyNotificationStateRepository()
    stateRepo.advanceResult = .failure(DomainError.networkUnavailable)
    let sut = makeSUT(notificationStateRepository: stateRepo)

    sut.advanceWatermark(asOf: Date())
    await sut.waitForPendingWatermarkAdvance()

    #expect(stateRepo.advanceCalls.count == 1)
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

  // MARK: - handlePushTap (integration of deep-link routing + watermark advance)

  @Test func handlePushTap_withApplicationRefAndCreatedAt_routesAndAdvances() async {
    // The end-to-end push-tap path: parser yields both a DeepLink and a
    // createdAt; the coordinator must route the deep link AND fire the
    // watermark advance with the parsed instant.
    let stateRepo = SpyNotificationStateRepository()
    let planningSpy = SpyPlanningApplicationRepository()
    planningSpy.fetchApplicationResult = .success(.permitted)
    let sut = AppCoordinator(
      repository: planningSpy,
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      notificationStateRepository: stateRepo
    )
    let userInfo: [AnyHashable: Any] = [
      "applicationRef": "APP-002",
      "createdAt": "2026-05-04T08:11:57Z",
    ]

    sut.handlePushTap(userInfo: userInfo)

    await sut.waitForPendingDetailLoad()
    await sut.waitForPendingWatermarkAdvance()

    #expect(planningSpy.fetchApplicationCalls == [PlanningApplicationId("APP-002")])
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime]
    #expect(stateRepo.advanceCalls == [formatter.date(from: "2026-05-04T08:11:57Z")])
  }

  @Test func handlePushTap_withApplicationRefAndNoCreatedAt_routesWithoutAdvance() async {
    // Older API builds (and any push that omits createdAt) must still deep-
    // link. Watermark advance is skipped when the timestamp is unavailable.
    let stateRepo = SpyNotificationStateRepository()
    let planningSpy = SpyPlanningApplicationRepository()
    planningSpy.fetchApplicationResult = .success(.permitted)
    let sut = AppCoordinator(
      repository: planningSpy,
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      notificationStateRepository: stateRepo
    )
    let userInfo: [AnyHashable: Any] = ["applicationRef": "APP-003"]

    sut.handlePushTap(userInfo: userInfo)

    await sut.waitForPendingDetailLoad()
    await sut.waitForPendingWatermarkAdvance()

    #expect(planningSpy.fetchApplicationCalls == [PlanningApplicationId("APP-003")])
    #expect(stateRepo.advanceCalls.isEmpty)
  }

  @Test func handlePushTap_digestPayload_doesNothing() async {
    // Digest pushes contain neither applicationRef nor a per-notification
    // createdAt. The coordinator must no-op rather than fire spurious
    // requests.
    let stateRepo = SpyNotificationStateRepository()
    let planningSpy = SpyPlanningApplicationRepository()
    let sut = AppCoordinator(
      repository: planningSpy,
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      notificationStateRepository: stateRepo
    )
    let userInfo: [AnyHashable: Any] = [
      "aps": ["alert": ["title": "Town Crier", "body": "5 new"], "badge": 5]
    ]

    sut.handlePushTap(userInfo: userInfo)

    await sut.waitForPendingDetailLoad()
    await sut.waitForPendingWatermarkAdvance()

    #expect(planningSpy.fetchApplicationCalls.isEmpty)
    #expect(stateRepo.advanceCalls.isEmpty)
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
}
