import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests covering the in-app notification preferences ViewModel which owns
/// *what* triggers a notification (saved-app push/email, weekly digest,
/// digest day) and exposes a read-only watch-zone count for navigation.
///
/// All four setters mirror `SettingsViewModel.persistSavedDecisionPreference`
/// — optimistic UI update, full-profile PATCH so unchanged fields round-trip,
/// rollback on failure.
@Suite("NotificationPreferencesViewModel")
@MainActor
struct NotificationPreferencesViewModelTests {

  private func makeSUT(
    profile: Result<ServerProfile, Error> = .success(.freeUser),
    zones: Result<[WatchZone], Error> = .success([]),
    authorizationStatus: NotificationAuthorizationStatus = .authorized
  ) -> (
    NotificationPreferencesViewModel,
    SpyUserProfileRepository,
    SpyWatchZoneRepository,
    SpyNotificationService
  ) {
    let profileSpy = SpyUserProfileRepository()
    profileSpy.createResult = profile
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = zones
    let notificationSpy = SpyNotificationService()
    notificationSpy.nextAuthorizationStatus = authorizationStatus
    let sut = NotificationPreferencesViewModel(
      userProfileRepository: profileSpy,
      watchZoneRepository: zoneSpy,
      notificationService: notificationSpy
    )
    return (sut, profileSpy, zoneSpy, notificationSpy)
  }

  private static func profile(
    pushEnabled: Bool = true,
    digestDay: DayOfWeek = .monday,
    emailDigestEnabled: Bool = true,
    savedDecisionPush: Bool = true,
    savedDecisionEmail: Bool = true
  ) -> ServerProfile {
    ServerProfile(
      userId: "auth0|user-001",
      tier: .personal,
      pushEnabled: pushEnabled,
      digestDay: digestDay,
      emailDigestEnabled: emailDigestEnabled,
      savedDecisionPush: savedDecisionPush,
      savedDecisionEmail: savedDecisionEmail
    )
  }

  private static func zone(name: String) throws -> WatchZone {
    try WatchZone(
      name: name,
      centre: Coordinate(latitude: 52.2, longitude: 0.12),
      radiusMetres: 1_000
    )
  }

  // MARK: - Load

  @Test func watchZoneCountIsNilBeforeLoad() {
    let (sut, _, _, _) = makeSUT()

    #expect(sut.watchZoneCount == nil)
  }

  @Test func loadPopulatesFieldsFromProfile() async {
    let (sut, _, _, _) = makeSUT(
      profile: .success(
        Self.profile(
          digestDay: .friday,
          emailDigestEnabled: false,
          savedDecisionPush: false,
          savedDecisionEmail: true
        )
      )
    )

    await sut.load()

    #expect(sut.savedDecisionPush == false)
    #expect(sut.savedDecisionEmail == true)
    #expect(sut.emailDigestEnabled == false)
    #expect(sut.digestDay == .friday)
    #expect(sut.error == nil)
  }

  @Test func loadPopulatesWatchZoneCount() async throws {
    let zones = [try Self.zone(name: "CB1 2AD"), try Self.zone(name: "CB2 1LA")]
    let (sut, _, _, _) = makeSUT(zones: .success(zones))

    await sut.load()

    #expect(sut.watchZoneCount == 2)
  }

  @Test func watchZoneCountStaysNilWhenZoneRepositoryThrows() async {
    let (sut, _, _, _) = makeSUT(
      profile: .success(.freeUser),
      zones: .failure(DomainError.networkUnavailable)
    )

    await sut.load()

    #expect(sut.watchZoneCount == nil)
  }

  @Test func loadFallsBackToDefaultsOnRepositoryThrow() async {
    let (sut, _, _, _) = makeSUT(
      profile: .failure(DomainError.networkUnavailable),
      zones: .failure(DomainError.networkUnavailable)
    )

    await sut.load()

    #expect(sut.savedDecisionPush == true)
    #expect(sut.savedDecisionEmail == true)
    #expect(sut.emailDigestEnabled == true)
    #expect(sut.digestDay == .monday)
    #expect(sut.watchZoneCount == nil)
  }

  // MARK: - Setters

  @Test func setSavedDecisionPushRoundTripsOtherFields() async {
    let initial = Self.profile(
      pushEnabled: true,
      digestDay: .friday,
      emailDigestEnabled: false,
      savedDecisionPush: true,
      savedDecisionEmail: true
    )
    let updated = Self.profile(
      pushEnabled: true,
      digestDay: .friday,
      emailDigestEnabled: false,
      savedDecisionPush: false,
      savedDecisionEmail: true
    )
    let (sut, profileSpy, _, _) = makeSUT(profile: .success(initial))
    profileSpy.updateResult = .success(updated)
    await sut.load()

    await sut.setSavedDecisionPush(false)

    #expect(profileSpy.updateCalls.count == 1)
    let call = profileSpy.updateCalls[0]
    #expect(call.pushEnabled == true)
    #expect(call.digestDay == .friday)
    #expect(call.emailDigestEnabled == false)
    #expect(call.savedDecisionPush == false)
    #expect(call.savedDecisionEmail == true)
    #expect(sut.savedDecisionPush == false)
  }

  @Test func setSavedDecisionEmailRoundTripsOtherFields() async {
    let initial = Self.profile(
      pushEnabled: false,
      digestDay: .wednesday,
      emailDigestEnabled: true,
      savedDecisionPush: true,
      savedDecisionEmail: true
    )
    let updated = Self.profile(
      pushEnabled: false,
      digestDay: .wednesday,
      emailDigestEnabled: true,
      savedDecisionPush: true,
      savedDecisionEmail: false
    )
    let (sut, profileSpy, _, _) = makeSUT(profile: .success(initial))
    profileSpy.updateResult = .success(updated)
    await sut.load()

    await sut.setSavedDecisionEmail(false)

    #expect(profileSpy.updateCalls.count == 1)
    let call = profileSpy.updateCalls[0]
    #expect(call.pushEnabled == false)
    #expect(call.digestDay == .wednesday)
    #expect(call.emailDigestEnabled == true)
    #expect(call.savedDecisionPush == true)
    #expect(call.savedDecisionEmail == false)
    #expect(sut.savedDecisionEmail == false)
  }

  @Test func setEmailDigestEnabledRoundTripsOtherFields() async {
    let initial = Self.profile(
      pushEnabled: true,
      digestDay: .tuesday,
      emailDigestEnabled: true,
      savedDecisionPush: false,
      savedDecisionEmail: true
    )
    let updated = Self.profile(
      pushEnabled: true,
      digestDay: .tuesday,
      emailDigestEnabled: false,
      savedDecisionPush: false,
      savedDecisionEmail: true
    )
    let (sut, profileSpy, _, _) = makeSUT(profile: .success(initial))
    profileSpy.updateResult = .success(updated)
    await sut.load()

    await sut.setEmailDigestEnabled(false)

    #expect(profileSpy.updateCalls.count == 1)
    let call = profileSpy.updateCalls[0]
    #expect(call.pushEnabled == true)
    #expect(call.digestDay == .tuesday)
    #expect(call.emailDigestEnabled == false)
    #expect(call.savedDecisionPush == false)
    #expect(call.savedDecisionEmail == true)
    #expect(sut.emailDigestEnabled == false)
  }

  @Test func setDigestDayRoundTripsOtherFields() async {
    let initial = Self.profile(
      pushEnabled: true,
      digestDay: .monday,
      emailDigestEnabled: true,
      savedDecisionPush: true,
      savedDecisionEmail: false
    )
    let updated = Self.profile(
      pushEnabled: true,
      digestDay: .saturday,
      emailDigestEnabled: true,
      savedDecisionPush: true,
      savedDecisionEmail: false
    )
    let (sut, profileSpy, _, _) = makeSUT(profile: .success(initial))
    profileSpy.updateResult = .success(updated)
    await sut.load()

    await sut.setDigestDay(.saturday)

    #expect(profileSpy.updateCalls.count == 1)
    let call = profileSpy.updateCalls[0]
    #expect(call.pushEnabled == true)
    #expect(call.digestDay == .saturday)
    #expect(call.emailDigestEnabled == true)
    #expect(call.savedDecisionPush == true)
    #expect(call.savedDecisionEmail == false)
    #expect(sut.digestDay == .saturday)
  }

  // MARK: - Rollback / error

  @Test func failedUpdateRollsBackOptimisticChange() async {
    let initial = Self.profile(
      pushEnabled: true,
      digestDay: .monday,
      emailDigestEnabled: true,
      savedDecisionPush: true,
      savedDecisionEmail: true
    )
    let (sut, profileSpy, _, _) = makeSUT(profile: .success(initial))
    profileSpy.updateResult = .failure(DomainError.networkUnavailable)
    await sut.load()

    await sut.setSavedDecisionPush(false)

    #expect(sut.savedDecisionPush == true)
    #expect(sut.savedDecisionEmail == true)
    #expect(sut.emailDigestEnabled == true)
    #expect(sut.digestDay == .monday)
  }

  @Test func failedUpdatePopulatesError() async {
    let (sut, profileSpy, _, _) = makeSUT(profile: .success(.freeUser))
    profileSpy.updateResult = .failure(DomainError.networkUnavailable)
    await sut.load()

    await sut.setEmailDigestEnabled(false)

    #expect(sut.error == .networkUnavailable)
  }

  // MARK: - Authorization Status

  @Test func authorizationStatusIsNilBeforeLoad() {
    let (sut, _, _, _) = makeSUT()

    #expect(sut.authorizationStatus == nil)
  }

  @Test func loadPopulatesAuthorizationStatus_notDetermined() async {
    let (sut, _, _, _) = makeSUT(authorizationStatus: .notDetermined)

    await sut.load()

    #expect(sut.authorizationStatus == .notDetermined)
  }

  @Test func loadPopulatesAuthorizationStatus_denied() async {
    let (sut, _, _, _) = makeSUT(authorizationStatus: .denied)

    await sut.load()

    #expect(sut.authorizationStatus == .denied)
  }

  @Test func loadPopulatesAuthorizationStatus_authorized() async {
    let (sut, _, _, _) = makeSUT(authorizationStatus: .authorized)

    await sut.load()

    #expect(sut.authorizationStatus == .authorized)
  }

  @Test func requestPermissionRefreshesAuthorizationStatus() async {
    let (sut, _, _, notificationSpy) = makeSUT(authorizationStatus: .notDetermined)
    await sut.load()
    notificationSpy.nextAuthorizationStatus = .authorized

    await sut.requestPermission()

    #expect(notificationSpy.requestPermissionCallCount == 1)
    #expect(sut.authorizationStatus == .authorized)
  }

  @Test func requestPermissionFailureSurfacesError() async {
    let (sut, _, _, notificationSpy) = makeSUT(authorizationStatus: .notDetermined)
    notificationSpy.requestPermissionResult = .failure(DomainError.networkUnavailable)
    await sut.load()
    notificationSpy.nextAuthorizationStatus = .denied

    await sut.requestPermission()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.authorizationStatus == .denied)
  }

  @Test func refreshAuthorizationStatusReQueriesService() async {
    let (sut, _, _, notificationSpy) = makeSUT(authorizationStatus: .notDetermined)
    await sut.load()
    let callsAfterLoad = notificationSpy.authorizationStatusCallCount
    notificationSpy.nextAuthorizationStatus = .authorized

    await sut.refreshAuthorizationStatus()

    #expect(notificationSpy.authorizationStatusCallCount == callsAfterLoad + 1)
    #expect(sut.authorizationStatus == .authorized)
  }
}
