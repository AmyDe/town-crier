import Foundation
import TownCrierDomain

/// ViewModel for the in-app notification preferences screen. Setters update
/// optimistically, PATCH the full profile so unchanged fields round-trip from
/// the cached server profile, and roll back on failure. ``pushEnabled`` is
/// round-tripped silently (not user-facing).
@MainActor
public final class NotificationPreferencesViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public private(set) var savedDecisionPush: Bool = true
  @Published public private(set) var savedDecisionEmail: Bool = true
  @Published public private(set) var emailDigestEnabled: Bool = true
  @Published public private(set) var digestDay: DayOfWeek = .monday
  @Published public private(set) var watchZoneCount: Int?
  /// Reflects the system notification permission. `nil` until the first
  /// `load()` completes — the view treats `nil` as "no section, no flash".
  @Published public private(set) var authorizationStatus: NotificationAuthorizationStatus?
  @Published public internal(set) var error: DomainError?

  /// Most recently loaded server profile. Source of truth for fields that
  /// each setter must round-trip unchanged when only one preference toggles.
  private var cachedServerProfile: ServerProfile?
  private var pushEnabled: Bool = true

  private let userProfileRepository: UserProfileRepository
  private let watchZoneRepository: WatchZoneRepository
  private let notificationService: NotificationService

  public init(
    userProfileRepository: UserProfileRepository,
    watchZoneRepository: WatchZoneRepository,
    notificationService: NotificationService
  ) {
    self.userProfileRepository = userProfileRepository
    self.watchZoneRepository = watchZoneRepository
    self.notificationService = notificationService
  }

  /// Loads the server profile and watch-zone count. On profile fetch failure
  /// the toggles fall back to their opted-in defaults and the cached profile is
  /// cleared so setters still PATCH safe defaults. Watch-zone count failures
  /// leave the count at zero.
  public func load() async {
    error = nil

    async let profileResult = loadProfile()
    async let zoneCount = loadZoneCount()
    async let status = notificationService.authorizationStatus()

    let profile = await profileResult
    let count = await zoneCount
    let resolvedStatus = await status

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
    authorizationStatus = resolvedStatus
  }

  /// Asks the system for notification permission, then re-reads the status
  /// even if the request throws, so "Don't allow" still produces `.denied`.
  public func requestPermission() async {
    error = nil
    do {
      _ = try await notificationService.requestPermission()
    } catch {
      handleError(error)
    }
    authorizationStatus = await notificationService.authorizationStatus()
  }

  /// Re-reads the system authorization status without prompting.
  public func refreshAuthorizationStatus() async {
    authorizationStatus = await notificationService.authorizationStatus()
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

  /// Toggle saved-application email notifications. See ``setSavedDecisionPush(_:)``
  /// for round-trip and rollback semantics.
  public func setSavedDecisionEmail(_ value: Bool) async {
    await persist(
      savedDecisionPush: savedDecisionPush,
      savedDecisionEmail: value,
      emailDigestEnabled: emailDigestEnabled,
      digestDay: digestDay
    )
  }

  /// Toggle the weekly email digest. See ``setSavedDecisionPush(_:)`` for
  /// round-trip and rollback semantics.
  public func setEmailDigestEnabled(_ value: Bool) async {
    await persist(
      savedDecisionPush: savedDecisionPush,
      savedDecisionEmail: savedDecisionEmail,
      emailDigestEnabled: value,
      digestDay: digestDay
    )
  }

  /// Set the day-of-week the weekly digest is sent on. See ``setSavedDecisionPush(_:)``
  /// for round-trip and rollback semantics.
  public func setDigestDay(_ value: DayOfWeek) async {
    await persist(
      savedDecisionPush: savedDecisionPush,
      savedDecisionEmail: savedDecisionEmail,
      emailDigestEnabled: emailDigestEnabled,
      digestDay: value
    )
  }

  private func loadProfile() async -> ServerProfile? {
    do {
      return try await userProfileRepository.create()
    } catch {
      return nil
    }
  }

  private func loadZoneCount() async -> Int? {
    do {
      return try await watchZoneRepository.loadAll().count
    } catch {
      return nil
    }
  }

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
