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
    let server =
      serverTierResolver ?? ServerTierResolver(userProfileRepository: userProfileRepository)
    self.serverTierResolver = server
    self.tierResolver =
      tierResolver
      ?? SubscriptionTierResolver(
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

    isLoading = false
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
  }
}
