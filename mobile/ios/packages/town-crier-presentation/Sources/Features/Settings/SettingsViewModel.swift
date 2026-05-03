import Combine
import Foundation
import TownCrierDomain

/// ViewModel managing the settings and account screen.
@MainActor
public final class SettingsViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public private(set) var userEmail: String?
  @Published public private(set) var userName: String?
  @Published public private(set) var authMethod: AuthMethod?
  @Published public private(set) var subscriptionTier: SubscriptionTier = .free
  @Published public private(set) var isTrialPeriod = false
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?
  @Published public var isShowingDeleteConfirmation = false
  @Published public var appearanceMode: AppearanceMode {
    didSet {
      defaults.set(appearanceMode.rawValue, forKey: Self.appearanceModeKey)
    }
  }
  /// Push notifications for decision updates on saved applications. Defaults to
  /// `true` to match the API's documented opt-out semantics — when no server
  /// profile is available the user is treated as opted in until they say
  /// otherwise.
  @Published public private(set) var savedDecisionPush: Bool = true
  /// Email notifications for decision updates on saved applications. Defaults
  /// to `true` for the same opt-out reasons as ``savedDecisionPush``.
  @Published public private(set) var savedDecisionEmail: Bool = true

  /// Most recently loaded server profile. Used as the source for fields that
  /// `update(...)` must round-trip unchanged when only one preference toggles
  /// (e.g. flipping `savedDecisionPush` should not also reset `digestDay`).
  private var cachedServerProfile: ServerProfile?

  public var onLogout: (() -> Void)?

  static let appearanceModeKey = "appearanceMode"

  private let authService: AuthenticationService
  private let subscriptionService: SubscriptionService
  private let userProfileRepository: UserProfileRepository
  private let serverTierResolver: ServerTierResolving
  private let tierResolver: SubscriptionTierResolving
  private let appVersionProvider: AppVersionProvider
  private let notificationService: NotificationService
  private let defaults: UserDefaults

  public init(
    authService: AuthenticationService,
    subscriptionService: SubscriptionService,
    userProfileRepository: UserProfileRepository,
    serverTierResolver: ServerTierResolving? = nil,
    tierResolver: SubscriptionTierResolving? = nil,
    appVersionProvider: AppVersionProvider,
    notificationService: NotificationService,
    defaults: UserDefaults = .standard
  ) {
    self.authService = authService
    self.subscriptionService = subscriptionService
    self.userProfileRepository = userProfileRepository
    let server = serverTierResolver ?? ServerTierResolver(userProfileRepository: userProfileRepository)
    self.serverTierResolver = server
    self.tierResolver = tierResolver ?? SubscriptionTierResolver(
      serverFetcher: { await server.ensureServerProfileTier() },
      storeKitFetcher: { await subscriptionService.currentEntitlement() },
      authService: authService
    )
    self.appVersionProvider = appVersionProvider
    self.notificationService = notificationService
    self.defaults = defaults

    let storedRaw = defaults.string(forKey: Self.appearanceModeKey) ?? ""
    self.appearanceMode = AppearanceMode(rawValue: storedRaw) ?? .system
  }

  public var appVersion: String {
    "\(appVersionProvider.version) (\(appVersionProvider.buildNumber))"
  }

  public var attributionItems: [AttributionItem] {
    [
      AttributionItem(
        name: "PlanIt",
        detail: "Planning application data",
        url: URL(string: "https://www.planit.org.uk")
      ),
      AttributionItem(
        name: "Crown Copyright",
        detail: "Contains public sector information"
      ),
      AttributionItem(
        name: "Ordnance Survey",
        detail: "Mapping data"
      ),
      AttributionItem(
        name: "Apple Maps",
        detail: "Map rendering and geocoding",
        url: URL(string: "https://www.apple.com/maps/")
      ),
    ]
  }

  public func load() async {
    isLoading = true
    error = nil

    var jwtTier: SubscriptionTier = .free
    var userSub: String?
    if let session = await authService.currentSession() {
      userEmail = session.userProfile.email
      userName = session.userProfile.name
      authMethod = session.userProfile.authMethod
      jwtTier = session.subscriptionTier
      userSub = session.userProfile.userId
    }

    let resolved = await tierResolver.resolve(
      jwtTier: jwtTier,
      previousTier: subscriptionTier,
      userSub: userSub
    )
    subscriptionTier = resolved.tier
    isTrialPeriod = resolved.isTrialPeriod

    await loadSavedDecisionPreferences()

    isLoading = false
  }

  /// Toggle the saved-application push preference and persist via
  /// `userProfileRepository.update(...)`. Other preference fields round-trip
  /// from the cached server profile so toggling one flag never silently
  /// rewrites another. On failure the published value rolls back so the UI
  /// reflects the server's last-known state.
  public func setSavedDecisionPush(_ value: Bool) async {
    await persistSavedDecisionPreference(
      savedDecisionPush: value,
      savedDecisionEmail: savedDecisionEmail
    )
  }

  /// Toggle the saved-application email preference. Mirrors
  /// ``setSavedDecisionPush(_:)`` — see its documentation for rollback and
  /// round-trip semantics.
  public func setSavedDecisionEmail(_ value: Bool) async {
    await persistSavedDecisionPreference(
      savedDecisionPush: savedDecisionPush,
      savedDecisionEmail: value
    )
  }

  public func logout() async {
    error = nil
    do {
      try? await notificationService.removeDeviceToken()
      try await authService.logout()
      clearSession()
      onLogout?()
    } catch {
      handleError(error) { .logoutFailed($0) }
    }
  }

  public func requestAccountDeletion() {
    isShowingDeleteConfirmation = true
  }

  public func cancelDeletion() {
    isShowingDeleteConfirmation = false
  }

  public func confirmDeleteAccount() async {
    isShowingDeleteConfirmation = false
    error = nil
    do {
      // UK GDPR Art. 17: server-side erasure must succeed BEFORE we drop the
      // local credentials. If we clear the keychain first and DELETE /v1/me
      // fails, the user's server data is orphaned and they can never retry.
      try await userProfileRepository.delete()
      try? await notificationService.removeDeviceToken()
      try await authService.deleteAccount()
      clearSession()
      onLogout?()
    } catch {
      handleError(error)
    }
  }

  private func clearSession() {
    userEmail = nil
    userName = nil
    authMethod = nil
    subscriptionTier = .free
    isTrialPeriod = false
    cachedServerProfile = nil
    savedDecisionPush = true
    savedDecisionEmail = true
  }

  /// Populates ``savedDecisionPush`` and ``savedDecisionEmail`` from the
  /// server profile. When the call fails (or no profile exists) both default
  /// to `true` to match the API's opt-out semantics — the user is treated as
  /// opted in until they explicitly toggle off.
  private func loadSavedDecisionPreferences() async {
    do {
      let profile = try await userProfileRepository.create()
      cachedServerProfile = profile
      savedDecisionPush = profile.savedDecisionPush
      savedDecisionEmail = profile.savedDecisionEmail
    } catch {
      cachedServerProfile = nil
      savedDecisionPush = true
      savedDecisionEmail = true
    }
  }

  /// Shared persistence path for the two saved-decision toggles. Callers pass
  /// the desired values for both flags; the unchanged one is supplied as the
  /// current published value so the server-side update is a single round trip.
  private func persistSavedDecisionPreference(
    savedDecisionPush nextPush: Bool,
    savedDecisionEmail nextEmail: Bool
  ) async {
    error = nil
    let previousPush = savedDecisionPush
    let previousEmail = savedDecisionEmail

    // Optimistic UI: reflect the desired value immediately, roll back on
    // failure. Mirrors the web counterpart (tc-so3a.17) which also flips the
    // toggle ahead of the network round trip.
    savedDecisionPush = nextPush
    savedDecisionEmail = nextEmail

    let pushEnabled = cachedServerProfile?.pushEnabled ?? true
    let digestDay = cachedServerProfile?.digestDay ?? .monday
    let emailDigestEnabled = cachedServerProfile?.emailDigestEnabled ?? true

    do {
      let updated = try await userProfileRepository.update(
        pushEnabled: pushEnabled,
        digestDay: digestDay,
        emailDigestEnabled: emailDigestEnabled,
        savedDecisionPush: nextPush,
        savedDecisionEmail: nextEmail
      )
      cachedServerProfile = updated
      savedDecisionPush = updated.savedDecisionPush
      savedDecisionEmail = updated.savedDecisionEmail
    } catch {
      savedDecisionPush = previousPush
      savedDecisionEmail = previousEmail
      handleError(error)
    }
  }
}
