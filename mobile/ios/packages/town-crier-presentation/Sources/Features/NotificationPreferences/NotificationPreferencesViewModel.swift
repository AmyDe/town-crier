import Foundation
import TownCrierDomain

/// ViewModel for the in-app notification preferences screen.
///
/// Owns *what* triggers a notification (saved-application push/email,
/// weekly email digest, digest day) and exposes a read-only watch-zone
/// count for the navigation row that links into the Zones tab.
///
/// Setters mirror `SettingsViewModel.persistSavedDecisionPreference` —
/// optimistic UI update, full-profile PATCH so unchanged fields round-trip
/// from the cached server profile, rollback to the previous value on
/// failure. ``pushEnabled`` is round-tripped silently (not user-facing).
@MainActor
public final class NotificationPreferencesViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public private(set) var savedDecisionPush: Bool = true
  @Published public private(set) var savedDecisionEmail: Bool = true
  @Published public private(set) var emailDigestEnabled: Bool = true
  @Published public private(set) var digestDay: DayOfWeek = .monday
  @Published public private(set) var watchZoneCount: Int = 0
  @Published public internal(set) var error: DomainError?

  /// Most recently loaded server profile. Source of truth for fields that
  /// each setter must round-trip unchanged when only one preference toggles.
  private var cachedServerProfile: ServerProfile?
  private var pushEnabled: Bool = true

  private let userProfileRepository: UserProfileRepository
  private let watchZoneRepository: WatchZoneRepository

  public init(
    userProfileRepository: UserProfileRepository,
    watchZoneRepository: WatchZoneRepository
  ) {
    self.userProfileRepository = userProfileRepository
    self.watchZoneRepository = watchZoneRepository
  }

  /// Loads the server profile and watch-zone count in parallel.
  ///
  /// On profile fetch failure, all four user-facing toggles fall back to
  /// `true`/defaults (the API's documented opt-out semantics — the user is
  /// treated as opted in until they say otherwise) and the cached profile
  /// is cleared so subsequent setters can still PATCH using safe defaults.
  /// Watch-zone count failures leave the count at zero.
  public func load() async {
    error = nil

    async let profileResult = loadProfile()
    async let zoneCount = loadZoneCount()

    let profile = await profileResult
    let count = await zoneCount

    if let profile {
      cachedServerProfile = profile
      savedDecisionPush = profile.savedDecisionPush
      savedDecisionEmail = profile.savedDecisionEmail
      emailDigestEnabled = profile.emailDigestEnabled
      digestDay = profile.digestDay
      pushEnabled = profile.pushEnabled
    } else {
      cachedServerProfile = nil
      savedDecisionPush = true
      savedDecisionEmail = true
      emailDigestEnabled = true
      digestDay = .monday
      pushEnabled = true
    }
    watchZoneCount = count
  }

  private func loadProfile() async -> ServerProfile? {
    do {
      return try await userProfileRepository.create()
    } catch {
      return nil
    }
  }

  private func loadZoneCount() async -> Int {
    do {
      return try await watchZoneRepository.loadAll().count
    } catch {
      return 0
    }
  }
}
