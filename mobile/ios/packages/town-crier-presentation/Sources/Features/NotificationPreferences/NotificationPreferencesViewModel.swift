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

  /// Toggle saved-application push notifications. Optimistic update with
  /// rollback on failure; the four other preference fields are sourced from
  /// `cachedServerProfile` so the PATCH always carries the full set.
  public func setSavedDecisionPush(_ value: Bool) async {
    await persist(
      savedDecisionPush: value,
      savedDecisionEmail: savedDecisionEmail,
      emailDigestEnabled: emailDigestEnabled,
      digestDay: digestDay
    )
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

  /// Shared persistence path for all four setters. Reflects the desired
  /// values immediately (optimistic UI), PATCHes the full preference set
  /// using `cachedServerProfile` for any field the caller hasn't changed,
  /// and rolls back the published values on failure.
  private func persist(
    savedDecisionPush nextSavedDecisionPush: Bool,
    savedDecisionEmail nextSavedDecisionEmail: Bool,
    emailDigestEnabled nextEmailDigestEnabled: Bool,
    digestDay nextDigestDay: DayOfWeek
  ) async {
    error = nil
    let previousSavedDecisionPush = savedDecisionPush
    let previousSavedDecisionEmail = savedDecisionEmail
    let previousEmailDigestEnabled = emailDigestEnabled
    let previousDigestDay = digestDay

    savedDecisionPush = nextSavedDecisionPush
    savedDecisionEmail = nextSavedDecisionEmail
    emailDigestEnabled = nextEmailDigestEnabled
    digestDay = nextDigestDay

    let pushEnabledForUpdate = cachedServerProfile?.pushEnabled ?? pushEnabled

    do {
      let updated = try await userProfileRepository.update(
        pushEnabled: pushEnabledForUpdate,
        digestDay: nextDigestDay,
        emailDigestEnabled: nextEmailDigestEnabled,
        savedDecisionPush: nextSavedDecisionPush,
        savedDecisionEmail: nextSavedDecisionEmail
      )
      cachedServerProfile = updated
      pushEnabled = updated.pushEnabled
      savedDecisionPush = updated.savedDecisionPush
      savedDecisionEmail = updated.savedDecisionEmail
      emailDigestEnabled = updated.emailDigestEnabled
      digestDay = updated.digestDay
    } catch {
      savedDecisionPush = previousSavedDecisionPush
      savedDecisionEmail = previousSavedDecisionEmail
      emailDigestEnabled = previousEmailDigestEnabled
      digestDay = previousDigestDay
      handleError(error)
    }
  }
}
