import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests for the home-tab push-permission nudge ViewModel (issue #624).
///
/// The banner is shown only when the user is on a paid tier and notifications
/// are not authorized. Its primary action branches on the system authorization
/// status: `.notDetermined` triggers the system prompt (then re-reads status),
/// `.denied` deep-links to iOS Settings (the prompt can never be re-shown once
/// denied). Status is refreshed on `scenePhase == .active` so the banner
/// disappears after the user enables notifications in iOS Settings and returns.
@Suite("PushNudgeViewModel")
@MainActor
struct PushNudgeViewModelTests {

  /// Records invocations of the injected `onOpenSettings` callback so tests can
  /// assert the `.denied` action deep-links rather than re-prompting.
  private final class OpenSettingsSpy {
    private(set) var callCount = 0
    func record() { callCount += 1 }
  }

  private func makeSUT(
    tier: SubscriptionTier = .personal,
    status: NotificationAuthorizationStatus = .notDetermined
  ) -> (PushNudgeViewModel, SpyNotificationService, OpenSettingsSpy) {
    let notificationSpy = SpyNotificationService()
    notificationSpy.nextAuthorizationStatus = status
    let openSettings = OpenSettingsSpy()
    let sut = PushNudgeViewModel(
      tier: tier,
      notificationService: notificationSpy
    ) { openSettings.record() }
    return (sut, notificationSpy, openSettings)
  }

  // MARK: - Visibility

  @Test func isNotVisibleBeforeLoad() {
    let (sut, _, _) = makeSUT(tier: .personal, status: .notDetermined)

    #expect(sut.isVisible == false)
  }

  @Test(
    "isVisible matrix: paid + unauthorized shows; authorized or free hides",
    arguments: [
      (
        tier: SubscriptionTier.personal, status: NotificationAuthorizationStatus.notDetermined,
        expected: true
      ),
      (tier: .personal, status: .denied, expected: true),
      (tier: .personal, status: .authorized, expected: false),
      (tier: .pro, status: .notDetermined, expected: true),
      (tier: .pro, status: .denied, expected: true),
      (tier: .pro, status: .authorized, expected: false),
      (tier: .free, status: .notDetermined, expected: false),
      (tier: .free, status: .denied, expected: false),
      (tier: .free, status: .authorized, expected: false),
    ]
  )
  func isVisibleMatrix(
    _ testCase: (tier: SubscriptionTier, status: NotificationAuthorizationStatus, expected: Bool)
  ) async {
    let (sut, _, _) = makeSUT(tier: testCase.tier, status: testCase.status)

    await sut.load()

    #expect(sut.isVisible == testCase.expected)
  }

  // MARK: - Copy

  @Test func notDeterminedCopyAsksToTurnOn() async {
    let (sut, _, _) = makeSUT(tier: .personal, status: .notDetermined)

    await sut.load()

    #expect(
      sut.bodyText
        == "Notifications are off. Turn them on to get the instant alerts you're paying for."
    )
    #expect(sut.buttonTitle == "Turn on")
  }

  @Test func deniedCopyPointsToSettings() async {
    let (sut, _, _) = makeSUT(tier: .personal, status: .denied)

    await sut.load()

    #expect(
      sut.bodyText
        == "Notifications are switched off in iOS Settings. Tap to turn them back on."
    )
    #expect(sut.buttonTitle == "Open Settings")
  }

  // MARK: - Primary action

  @Test func notDeterminedActionRequestsPermissionThenReReadsStatus() async {
    let (sut, notificationSpy, openSettings) = makeSUT(
      tier: .personal,
      status: .notDetermined
    )
    await sut.load()
    notificationSpy.nextAuthorizationStatus = .authorized

    await sut.primaryAction()

    #expect(notificationSpy.requestPermissionCallCount == 1)
    #expect(openSettings.callCount == 0)
    #expect(sut.authorizationStatus == .authorized)
    #expect(sut.isVisible == false)
  }

  @Test func deniedActionOpensSettingsAndDoesNotRequestPermission() async {
    let (sut, notificationSpy, openSettings) = makeSUT(
      tier: .personal,
      status: .denied
    )
    await sut.load()

    await sut.primaryAction()

    #expect(openSettings.callCount == 1)
    #expect(notificationSpy.requestPermissionCallCount == 0)
  }

  // MARK: - Foreground refresh

  @Test func refreshHidesBannerWhenStatusBecomesAuthorized() async {
    let (sut, notificationSpy, _) = makeSUT(tier: .personal, status: .denied)
    await sut.load()
    #expect(sut.isVisible == true)
    notificationSpy.nextAuthorizationStatus = .authorized

    await sut.refresh()

    #expect(sut.authorizationStatus == .authorized)
    #expect(sut.isVisible == false)
  }
}
