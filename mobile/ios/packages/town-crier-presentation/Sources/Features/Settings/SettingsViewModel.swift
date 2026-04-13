import Combine
import Foundation
import TownCrierDomain
import os

/// ViewModel managing the settings and account screen.
@MainActor
public final class SettingsViewModel: ObservableObject, ErrorHandlingViewModel {
  private static let logger = Logger(subsystem: "uk.towncrierapp", category: "SettingsViewModel")
  @Published public private(set) var userEmail: String?
  @Published public private(set) var userName: String?
  @Published public private(set) var authMethod: AuthMethod?
  @Published public private(set) var subscriptionTier: SubscriptionTier = .free
  @Published public private(set) var isTrialPeriod = false
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?
  @Published public var isShowingDeleteConfirmation = false

  public var onLogout: (() -> Void)?

  private let authService: AuthenticationService
  private let subscriptionService: SubscriptionService
  private let userProfileRepository: UserProfileRepository
  private let appVersionProvider: AppVersionProvider
  private let notificationService: NotificationService

  public init(
    authService: AuthenticationService,
    subscriptionService: SubscriptionService,
    userProfileRepository: UserProfileRepository,
    appVersionProvider: AppVersionProvider,
    notificationService: NotificationService
  ) {
    self.authService = authService
    self.subscriptionService = subscriptionService
    self.userProfileRepository = userProfileRepository
    self.appVersionProvider = appVersionProvider
    self.notificationService = notificationService
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
        name: "OpenStreetMap",
        detail: "Map tiles and geodata",
        url: URL(string: "https://www.openstreetmap.org")
      ),
    ]
  }

  public func load() async {
    isLoading = true
    error = nil

    var jwtTier: SubscriptionTier = .free
    if let session = await authService.currentSession() {
      userEmail = session.userProfile.email
      userName = session.userProfile.name
      authMethod = session.userProfile.authMethod
      jwtTier = session.subscriptionTier
    }

    let resolvedTier = await resolveSubscriptionTier(jwtTier: jwtTier)
    subscriptionTier = resolvedTier.tier
    isTrialPeriod = resolvedTier.isTrialPeriod

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
      try? await notificationService.removeDeviceToken()
      try await authService.deleteAccount()
      clearSession()
      onLogout?()
    } catch {
      handleError(error)
    }
  }

  /// Resolves the subscription tier by consulting the backend API profile
  /// (source of truth), StoreKit (for App Store purchases), and the JWT
  /// access token claim, then picking the highest tier. This handles
  /// web-purchased subscriptions that StoreKit does not know about, recently
  /// App-Store-purchased subscriptions that the server may not have synced
  /// yet, and API failures where the JWT claim provides a viable fallback.
  private func resolveSubscriptionTier(
    jwtTier: SubscriptionTier
  ) async -> (tier: SubscriptionTier, isTrialPeriod: Bool) {
    let serverTier = await fetchServerTier()
    let storeKitEntitlement = await subscriptionService.currentEntitlement()

    let storeKitTier = storeKitEntitlement?.tier ?? .free
    let highestTier = max(serverTier, max(storeKitTier, jwtTier))

    // Only report trial status from StoreKit — the server profile doesn't
    // carry trial information. Trial period is only meaningful when the
    // StoreKit tier is the one that won.
    let isTrialPeriod =
      storeKitEntitlement?.isTrialPeriod == true && storeKitTier >= max(serverTier, jwtTier)

    return (highestTier, isTrialPeriod)
  }

  private func fetchServerTier() async -> SubscriptionTier {
    do {
      if let profile = try await userProfileRepository.fetch() {
        return profile.tier
      }
      return .free
    } catch {
      Self.logger.error(
        "Failed to fetch server profile for subscription tier: \(error.localizedDescription)"
      )
      return .free
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
