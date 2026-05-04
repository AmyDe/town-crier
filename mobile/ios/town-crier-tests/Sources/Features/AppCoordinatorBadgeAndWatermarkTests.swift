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
