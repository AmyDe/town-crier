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
    zones: Result<[WatchZone], Error> = .success([])
  ) -> (NotificationPreferencesViewModel, SpyUserProfileRepository, SpyWatchZoneRepository) {
    let profileSpy = SpyUserProfileRepository()
    profileSpy.createResult = profile
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = zones
    let sut = NotificationPreferencesViewModel(
      userProfileRepository: profileSpy,
      watchZoneRepository: zoneSpy
    )
    return (sut, profileSpy, zoneSpy)
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

  @Test func loadPopulatesFieldsFromProfile() async {
    let (sut, _, _) = makeSUT(
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
    let (sut, _, _) = makeSUT(zones: .success(zones))

    await sut.load()

    #expect(sut.watchZoneCount == 2)
  }

  @Test func loadFallsBackToDefaultsOnRepositoryThrow() async {
    let (sut, _, _) = makeSUT(
      profile: .failure(DomainError.networkUnavailable),
      zones: .failure(DomainError.networkUnavailable)
    )

    await sut.load()

    #expect(sut.savedDecisionPush == true)
    #expect(sut.savedDecisionEmail == true)
    #expect(sut.emailDigestEnabled == true)
    #expect(sut.digestDay == .monday)
    #expect(sut.watchZoneCount == 0)
  }
}
