import Combine
import Foundation
import TownCrierDomain
import os

/// Root coordinator managing top-level navigation.
@MainActor
public final class AppCoordinator: ObservableObject {
  private static let logger = Logger(subsystem: "uk.towncrierapp", category: "AppCoordinator")

  @Published public var detailApplication: PlanningApplication?
  @Published public var deepLinkError: DomainError?
  @Published public var presentedLegalDocument: LegalDocumentType?
  @Published public var isManageSubscriptionPresented = false
  @Published public var isSubscriptionPresented = false
  /// Set to `true` from the in-app preferences screen footer; the view layer
  /// opens `UIApplication/openSettingsURLString` and resets the flag.
  @Published public var isOpeningSystemNotificationSettings = false
  /// In-app preferences: `true` pushes `NotificationPreferencesView` via `.navigationDestination`.
  @Published public var isNotificationPreferencesPresented = false
  /// Selected main tab; bound to the root `TabView` for coordinator-driven tab switches.
  @Published public var selectedTab: MainTab = .applications
  @Published public var isAddingWatchZone = false
  @Published public var editingWatchZone: WatchZone?
  @Published public var isRedeemOfferCodePresented = false
  /// Settings sheet — bound to the gear-icon toolbar action installed on each tab.
  @Published public var isSettingsPresented = false
  @Published public private(set) var subscriptionTier: SubscriptionTier = .free

  public var isOnboardingComplete: Bool {
    onboardingRepository.isOnboardingComplete
  }

  private static let tierCacheKey = "cachedSubscriptionTier"

  private let repository: PlanningApplicationRepository
  private let authService: AuthenticationService
  private let subscriptionService: SubscriptionService
  let userProfileRepository: UserProfileRepository
  private let serverTierResolver: ServerTierResolving
  private let tierResolver: SubscriptionTierResolving
  private let onboardingRepository: OnboardingRepository
  private let notificationService: NotificationService
  private let offlineRepository: OfflineAwareRepository?
  let authorityRepository: ApplicationAuthorityRepository?
  let watchZoneRepository: WatchZoneRepository
  private let geocoder: PostcodeGeocoder?
  private let appVersionProvider: AppVersionProvider
  private let versionConfigService: VersionConfigService
  private let savedApplicationRepository: SavedApplicationRepository?
  private let offerCodeService: OfferCodeService?
  private let tierCache: UserDefaults
  // Cached strongly so SwiftUI's factory re-evaluation doesn't leave the
  // coordinator with a dangling reference; editor `onSave` needs a live VM.
  private var watchZoneListViewModel: WatchZoneListViewModel?
  // In-flight tasks tests can await deterministically (no `Task.sleep`).
  private var pendingOfferCodeRefresh: Task<Void, Never>?
  private var pendingWatchZoneRefresh: Task<Void, Never>?
  private var pendingDetailLoad: Task<Void, Never>?

  public init(
    repository: PlanningApplicationRepository,
    authService: AuthenticationService,
    subscriptionService: SubscriptionService,
    userProfileRepository: UserProfileRepository,
    serverTierResolver: ServerTierResolving? = nil,
    tierResolver: SubscriptionTierResolving? = nil,
    offlineRepository: OfflineAwareRepository? = nil,
    authorityRepository: ApplicationAuthorityRepository? = nil,
    watchZoneRepository: WatchZoneRepository,
    geocoder: PostcodeGeocoder? = nil,
    onboardingRepository: OnboardingRepository,
    notificationService: NotificationService,
    appVersionProvider: AppVersionProvider,
    versionConfigService: VersionConfigService,
    savedApplicationRepository: SavedApplicationRepository? = nil,
    offerCodeService: OfferCodeService? = nil,
    tierCache: UserDefaults? = nil
  ) {
    self.repository = repository
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
    self.offlineRepository = offlineRepository
    self.authorityRepository = authorityRepository
    self.watchZoneRepository = watchZoneRepository
    self.geocoder = geocoder
    self.onboardingRepository = onboardingRepository
    self.notificationService = notificationService
    self.appVersionProvider = appVersionProvider
    self.versionConfigService = versionConfigService
    self.savedApplicationRepository = savedApplicationRepository
    self.offerCodeService = offerCodeService
    self.tierCache = tierCache ?? .standard

    // Restore the last successfully resolved tier so that paying users
    // retain feature access immediately, even before the live resolution
    // completes (or when it fails on simulator).
    if let cached = self.tierCache.string(forKey: Self.tierCacheKey),
      let tier = SubscriptionTier(rawValue: cached) {
      subscriptionTier = tier
    }
  }

  public func makeLoginViewModel() -> LoginViewModel {
    LoginViewModel(authService: authService)
  }

  public func makeMapViewModel() -> MapViewModel {
    MapViewModel(
      repository: repository,
      watchZoneRepository: watchZoneRepository,
      savedApplicationRepository: savedApplicationRepository
    )
  }

  public func makeApplicationListViewModel(
    zone: WatchZone
  ) -> ApplicationListViewModel {
    let viewModel: ApplicationListViewModel
    if let offlineRepository {
      viewModel = ApplicationListViewModel(
        offlineRepository: offlineRepository,
        zone: zone
      )
    } else {
      viewModel = ApplicationListViewModel(
        repository: repository,
        zone: zone
      )
    }
    viewModel.onApplicationSelected = { [weak self] id in
      self?.showApplicationDetail(id)
    }
    return viewModel
  }

  /// Creates a list view model that resolves the user's first watch zone at load time.
  /// Callers should prefer `makeApplicationListViewModel(zone:)` when a specific zone is known.
  public func makeApplicationListViewModel() -> ApplicationListViewModel {
    let viewModel: ApplicationListViewModel
    if let offlineRepository {
      viewModel = ApplicationListViewModel(
        watchZoneRepository: watchZoneRepository,
        offlineRepository: offlineRepository
      )
    } else {
      viewModel = ApplicationListViewModel(
        watchZoneRepository: watchZoneRepository,
        repository: repository
      )
    }
    viewModel.onApplicationSelected = { [weak self] id in
      self?.showApplicationDetail(id)
    }
    return viewModel
  }

  /// Factory for the dedicated Saved tab view model. Falls back to a no-op
  /// repository when no `SavedApplicationRepository` was injected — the Saved
  /// tab is meaningful only when bookmarking is wired, but a fatal error here
  /// would crash users who never tap the tab.
  public func makeSavedApplicationListViewModel() -> SavedApplicationListViewModel {
    let repository = savedApplicationRepository ?? UnavailableSavedApplicationRepository()
    let viewModel = SavedApplicationListViewModel(savedApplicationRepository: repository)
    viewModel.onApplicationSelected = { [weak self] id in
      self?.showApplicationDetail(id)
    }
    viewModel.onApplicationSelectedWithPayload = { [weak self] application in
      self?.showApplicationDetail(application)
    }
    return viewModel
  }

  public func makeSettingsViewModel() -> SettingsViewModel {
    SettingsViewModel(
      authService: authService,
      subscriptionService: subscriptionService,
      userProfileRepository: userProfileRepository,
      appVersionProvider: appVersionProvider,
      notificationService: notificationService
    )
  }

  public func makeForceUpdateViewModel() -> ForceUpdateViewModel {
    ForceUpdateViewModel(
      versionConfigService: versionConfigService,
      appVersionProvider: appVersionProvider
    )
  }

  public func makeApplicationDetailViewModel(
    application: PlanningApplication
  ) -> ApplicationDetailViewModel {
    let viewModel = ApplicationDetailViewModel(
      application: application,
      savedApplicationRepository: savedApplicationRepository,
      planningApplicationRepository: repository
    )
    viewModel.onDismiss = { [weak self] in
      self?.detailApplication = nil
    }
    return viewModel
  }

  // MARK: - Subscription Tier Resolution

  /// Resolves the subscription tier via the shared ``SubscriptionTierResolver``
  /// so this coordinator and ``SettingsViewModel`` cannot drift apart again
  /// (third recurrence after tc-aza5; original bug tc-exg6).
  public func resolveSubscriptionTier() async {
    let session = await authService.currentSession()
    let result = await tierResolver.resolve(
      jwtTier: session?.subscriptionTier ?? .free,
      previousTier: subscriptionTier,
      userSub: session?.userProfile.userId
    )
    subscriptionTier = result.tier
    tierCache.set(result.tier.rawValue, forKey: Self.tierCacheKey)
  }

  // MARK: - Watch Zone Factories

  public func makeWatchZoneListViewModel() -> WatchZoneListViewModel {
    if let cached = watchZoneListViewModel {
      return cached
    }
    let viewModel = WatchZoneListViewModel(
      repository: watchZoneRepository,
      featureGate: FeatureGate(tier: subscriptionTier)
    )
    viewModel.onAddZone = { [weak self] in
      self?.isAddingWatchZone = true
    }
    viewModel.onEditZone = { [weak self] zone in
      self?.editingWatchZone = zone
    }
    viewModel.onViewPlans = { [weak self] in
      self?.isSubscriptionPresented = true
    }
    watchZoneListViewModel = viewModel
    return viewModel
  }

  public func makeWatchZoneEditorViewModel(
    editing zone: WatchZone? = nil
  ) -> WatchZoneEditorViewModel {
    guard let geocoder else {
      fatalError("PostcodeGeocoder must be injected to create WatchZoneEditorViewModel")
    }
    let viewModel = WatchZoneEditorViewModel(
      geocoder: geocoder,
      repository: watchZoneRepository,
      tier: subscriptionTier,
      editing: zone
    )
    let isEditing = zone != nil
    viewModel.onSave = { [weak self] saved in
      self?.isAddingWatchZone = false
      self?.editingWatchZone = nil
      self?.pendingWatchZoneRefresh = Task { [weak self] in
        // Invalidate the per-zone applications cache before reloading so a
        // radius/centre change does not serve a stale cache hit on the
        // Apps view for up to the cache TTL (tc-9vid).
        if isEditing, let offlineRepository = self?.offlineRepository {
          await offlineRepository.invalidateCache(for: saved.id)
        }
        await self?.watchZoneListViewModel?.load()
      }
    }
    return viewModel
  }

  /// Test-only synchronisation: await the most recently kicked-off post-save
  /// watch-zone reload so assertions happen after the list view-model has
  /// been refreshed against the latest repository state. Replaces flaky
  /// `Task.sleep(...)` waits in tests.
  public func waitForPendingWatchZoneRefresh() async {
    await pendingWatchZoneRefresh?.value
  }

  // MARK: - Settings Navigation

  public func showPrivacyPolicy() {
    presentedLegalDocument = .privacyPolicy
  }

  public func showTermsOfService() {
    presentedLegalDocument = .termsOfService
  }

  public func showManageSubscription() {
    isManageSubscriptionPresented = true
  }

  /// Presents the Settings view as a sheet from any tab. Bound to the gear
  /// icon installed via the `.settingsToolbar` ViewModifier.
  public func showSettings() {
    isSettingsPresented = true
  }

  /// Deep-links to iOS system Settings (push permissions). Coordinator stays
  /// UIKit-free; `TownCrierApp` observes the flag and opens the settings URL.
  public func showSystemNotificationSettings() {
    isOpeningSystemNotificationSettings = true
  }

  // MARK: - Offer Codes

  /// Presents the "Redeem Offer Code" sheet from Settings. Has no effect if
  /// the Coordinator was constructed without an `OfferCodeService` (i.e. the
  /// feature is not wired).
  public func showRedeemOfferCode() {
    guard offerCodeService != nil else { return }
    isRedeemOfferCodePresented = true
  }

  /// Creates a `RedeemOfferCodeViewModel` wired to dismiss the sheet and
  /// refresh the subscription tier on successful redemption.
  ///
  /// Returns `nil` when no `OfferCodeService` was injected — callers should
  /// hide the Settings entry point in that case.
  public func makeRedeemOfferCodeViewModel() -> RedeemOfferCodeViewModel? {
    guard let offerCodeService else { return nil }
    let viewModel = RedeemOfferCodeViewModel(offerCodeService: offerCodeService)
    viewModel.onRedeemed = { [weak self] _ in
      self?.handleOfferCodeRedeemed()
    }
    return viewModel
  }

  /// Test-only synchronisation: await the post-redemption refresh so
  /// assertions happen after the session and tier have been re-resolved.
  public func waitForPendingOfferCodeRefresh() async {
    await pendingOfferCodeRefresh?.value
  }

  private func handleOfferCodeRedeemed() {
    isRedeemOfferCodePresented = false
    // Detached task so tests can await it. Session refresh rotates the JWT so
    // the next server call sees the new `subscription_tier` claim, then
    // re-resolving the tier picks up the updated profile.
    pendingOfferCodeRefresh = Task { [weak self] in
      guard let self else { return }
      do {
        _ = try await authService.refreshSession()
      } catch {
        Self.logger.error(
          "Offer-code session refresh failed: \(error.localizedDescription)"
        )
      }
      await resolveSubscriptionTier()
    }
  }

  public func handleDeepLink(_ deepLink: DeepLink) {
    deepLinkError = nil
    switch deepLink {
    case .applicationDetail(let id):
      showApplicationDetail(id)
    }
  }

  /// Presents the detail sheet synchronously from a row payload — bypasses the
  /// per-id fetch so the sheet appears instantly. The detail view model still
  /// runs `refresh()` in `.task` to keep saved-row snapshots fresh on the
  /// server (bd tc-sslz, tc-udby).
  func showApplicationDetail(_ application: PlanningApplication) {
    detailApplication = application
  }

  func showApplicationDetail(_ id: PlanningApplicationId) {
    pendingDetailLoad = Task { [weak self] in
      guard let self else { return }
      do {
        detailApplication = try await repository.fetchApplication(by: id)
      } catch let domainError as DomainError {
        deepLinkError = domainError
      } catch {
        deepLinkError = .unexpected(error.localizedDescription)
      }
    }
  }

  /// Test-only synchronisation: await the most recent
  /// `showApplicationDetail` fetch. Replaces flaky `Task.sleep` waits.
  public func waitForPendingDetailLoad() async {
    await pendingDetailLoad?.value
  }
}
