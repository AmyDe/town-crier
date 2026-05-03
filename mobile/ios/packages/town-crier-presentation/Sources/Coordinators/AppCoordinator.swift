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
  /// Toggled to `true` when the user taps "Notification Preferences"; the view
  /// layer opens `UIApplication/openSettingsURLString` and resets the flag.
  /// Keeps the coordinator UIKit-free while staying testable.
  @Published public var isOpeningSystemNotificationSettings = false
  @Published public var isAddingWatchZone = false
  @Published public var editingWatchZone: WatchZone?
  @Published public var isRedeemOfferCodePresented = false
  /// Toggled to `true` when the user taps the gear icon on any tab. The view
  /// layer presents the Settings view as a sheet bound to this flag.
  @Published public var isSettingsPresented = false
  @Published public private(set) var subscriptionTier: SubscriptionTier = .free

  public var isOnboardingComplete: Bool {
    onboardingRepository.isOnboardingComplete
  }

  private static let tierCacheKey = "cachedSubscriptionTier"

  private let repository: PlanningApplicationRepository
  private let authService: AuthenticationService
  private let subscriptionService: SubscriptionService
  private let userProfileRepository: UserProfileRepository
  private let serverTierResolver: ServerTierResolving
  private let onboardingRepository: OnboardingRepository
  private let notificationService: NotificationService
  private let offlineRepository: OfflineAwareRepository?
  let authorityRepository: ApplicationAuthorityRepository?
  private let watchZoneRepository: WatchZoneRepository
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
    self.serverTierResolver =
      serverTierResolver ?? ServerTierResolver(userProfileRepository: userProfileRepository)
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

  /// Factory for the dedicated Saved tab view model.
  ///
  /// Falls back to a no-op repository when no `SavedApplicationRepository` was
  /// injected. The Saved tab is only meaningful when bookmarking is wired, so
  /// in production this branch is unreachable; surfacing a fatal error there
  /// would crash the app for users who never tap the tab.
  public func makeSavedApplicationListViewModel() -> SavedApplicationListViewModel {
    let repository = savedApplicationRepository ?? UnavailableSavedApplicationRepository()
    let viewModel = SavedApplicationListViewModel(savedApplicationRepository: repository)
    viewModel.onApplicationSelected = { [weak self] id in
      self?.showApplicationDetail(id)
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

  /// Resolves the subscription tier by consulting JWT, StoreKit and the
  /// server profile, then picks the highest tier (mirrors ``SettingsViewModel``).
  /// When the server fetch fails (nil), the previously resolved tier is
  /// preserved so paying users don't lose feature access due to transient errors
  /// — common on the simulator where JWT and StoreKit always return `.free`.
  public func resolveSubscriptionTier() async {
    var jwtTier: SubscriptionTier = .free
    if let session = await authService.currentSession() {
      jwtTier = session.subscriptionTier
    }

    let serverTier = await serverTierResolver.ensureServerProfileTier()
    let storeKitTier = await subscriptionService.currentEntitlement()?.tier ?? .free

    // When the server profile ensure-or-fetch call failed (nil), fall back to
    // the current tier so we don't downgrade a paying user due to a transient
    // error.
    let effectiveServerTier = serverTier ?? subscriptionTier
    let resolved = max(effectiveServerTier, max(storeKitTier, jwtTier))
    subscriptionTier = resolved
    tierCache.set(resolved.rawValue, forKey: Self.tierCacheKey)
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

  /// Requests the view layer deep-link to the iOS system Settings page for
  /// the app (push permissions: banners, sounds, badges, focus, etc.). The
  /// Coordinator stays UIKit-free; `TownCrierApp` observes the flag and
  /// performs the actual ``UIApplication/openSettingsURLString`` open.
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

  /// Test-only synchronisation: await the most recently kicked-off
  /// post-redemption refresh so assertions happen after the session has been
  /// rotated and the tier re-resolved.
  public func waitForPendingOfferCodeRefresh() async {
    await pendingOfferCodeRefresh?.value
  }

  private func handleOfferCodeRedeemed() {
    isRedeemOfferCodePresented = false
    // Kick off the refresh on a detached task so we can await it in tests.
    // Session refresh rotates the JWT so the next server call sees the new
    // `subscription_tier` claim; re-resolving the tier pulls the updated
    // profile and updates `subscriptionTier` for all tier-gated views.
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

  /// Presents the detail sheet from a list row that already has the full
  /// payload — bypasses the per-id fetch so the sheet appears instantly. The
  /// detail view model still calls `refresh()` in `.task` to keep the
  /// saved-row snapshot fresh on the server (bd tc-sslz, tc-udby).
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
