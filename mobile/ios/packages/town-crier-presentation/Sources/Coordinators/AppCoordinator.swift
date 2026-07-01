import Combine
import Foundation
import TownCrierDomain
import os

/// Root coordinator managing top-level navigation.
@MainActor
public final class AppCoordinator: ObservableObject {
  static let logger = Logger(subsystem: "uk.towncrierapp", category: "AppCoordinator")

  @Published public var detailApplication: PlanningApplication?
  @Published public var deepLinkError: DomainError?
  @Published public var presentedLegalDocument: LegalDocumentType?
  @Published public var isManageSubscriptionPresented = false
  @Published public var isSubscriptionPresented = false
  /// Set to `true` from the in-app preferences screen footer; the view layer
  /// opens ``AppCoordinator/systemNotificationSettingsURLString`` and resets it.
  @Published public var isOpeningSystemNotificationSettings = false
  @Published public var isOpeningAppStoreReview = false  // see rateApp() (GH #629)
  /// In-app preferences: `true` pushes `NotificationPreferencesView` via `.navigationDestination`.
  @Published public var isNotificationPreferencesPresented = false
  /// Set to `true` by the review-prompt requester at an engagement peak; the app
  /// layer observes it, calls SwiftUI's `requestReview`, then resets it
  /// (mirroring ``isOpeningSystemNotificationSettings``). The OS call is never
  /// guaranteed to show — Apple caps and may silently suppress it (GH #628).
  @Published public var isRequestingReview = false
  /// Selected main tab; bound to the root `TabView` for coordinator-driven tab switches.
  @Published public var selectedTab: MainTab = .applications
  @Published public var isAddingWatchZone = false
  @Published public var editingWatchZone: WatchZone?
  @Published public var isRedeemOfferCodePresented = false
  /// Settings sheet — bound to the gear-icon toolbar action installed on each tab.
  @Published public var isSettingsPresented = false
  @Published public private(set) var subscriptionTier: SubscriptionTier = .free
  /// Drives the root view's choice between the onboarding wizard, a loading
  /// screen, and the main tab view (tc-w3cb.1). Seeded from the device-local
  /// completion latch as a fast path so returning users skip the loading
  /// screen, then confirmed authoritatively against account state by
  /// ``determineOnboarding()``.
  @Published public internal(set) var onboardingPresentation: OnboardingPresentation = .undetermined

  public var isOnboardingComplete: Bool {
    onboardingRepository.isOnboardingComplete
  }

  private static let tierCacheKey = "cachedSubscriptionTier"

  // Internal (not private) so the AppCoordinator+Detail extension can read it.
  let repository: PlanningApplicationRepository
  let authService: AuthenticationService
  private let subscriptionService: SubscriptionService
  let userProfileRepository: UserProfileRepository
  private let tierResolver: SubscriptionTierResolving
  // Internal (not private) so the AppCoordinator+Onboarding extension can read it.
  let onboardingRepository: OnboardingRepository
  let notificationService: NotificationService
  // Internal (not private) so the AppCoordinator+WatchZones extension can read it.
  let offlineRepository: OfflineAwareRepository?
  let authorityRepository: ApplicationAuthorityRepository?
  let watchZoneRepository: WatchZoneRepository
  // Internal (not private) so the AppCoordinator+Onboarding extension can read it.
  let geocoder: PostcodeGeocoder?
  private let appVersionProvider: AppVersionProvider
  private let versionConfigService: VersionConfigService
  private let savedApplicationRepository: SavedApplicationRepository?
  let offerCodeService: OfferCodeService?
  private let tierCache: UserDefaults
  let notificationStateRepository: NotificationStateRepository?
  let badgeSetter: BadgeSetting?
  // Drives the App Store review prompt at engagement peaks (GH #628). Optional
  // so existing call sites and tests that don't exercise it inject nothing.
  let reviewPromptTracker: ReviewPromptTracker?
  // Cached strongly so SwiftUI's factory re-evaluation doesn't leave the
  // coordinator with a dangling reference; editor `onSave` needs a live VM.
  // Internal (not private) so the AppCoordinator+WatchZones extension owns it.
  var watchZoneListViewModel: WatchZoneListViewModel?
  // Retained so the live subscription tier can be pushed into the wizard in
  // place (rather than rebuilding it and losing in-progress postcode/geocode),
  // and so SwiftUI's StateObject and the coordinator converge on one instance.
  // Internal (not private) so the AppCoordinator+Onboarding extension owns it.
  var onboardingViewModel: OnboardingViewModel?
  // In-flight tasks tests can await deterministically (no `Task.sleep`).
  var pendingOfferCodeRefresh: Task<Void, Never>?
  // Internal (not private) so the AppCoordinator+WatchZones extension can drive it.
  var pendingWatchZoneRefresh: Task<Void, Never>?
  var pendingDetailLoad: Task<Void, Never>?
  // Push-tap per-application mark-read — fire-and-forget but stored so tests
  // can await deterministically (tc-0sfx.3, ADR 0035).
  var pendingApplicationMarkRead: Task<Void, Never>?
  // Foreground badge sync (tc-1nsa.9).
  var pendingBadgeSync: Task<Void, Never>?
  // Post-purchase push-permission prompt — fire-and-forget but stored so
  // tests can await deterministically (issue #624, Prong 1).
  var pendingPostPurchasePermissionPrompt: Task<Void, Never>?

  public init(
    repository: PlanningApplicationRepository,
    authService: AuthenticationService,
    subscriptionService: SubscriptionService,
    userProfileRepository: UserProfileRepository,
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
    tierCache: UserDefaults? = nil,
    notificationStateRepository: NotificationStateRepository? = nil,
    badgeSetter: BadgeSetting? = nil,
    reviewPromptTracker: ReviewPromptTracker? = nil
  ) {
    self.repository = repository
    self.authService = authService
    self.subscriptionService = subscriptionService
    self.userProfileRepository = userProfileRepository
    let server = ServerTierResolver(userProfileRepository: userProfileRepository)
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
    self.notificationStateRepository = notificationStateRepository
    self.badgeSetter = badgeSetter
    self.reviewPromptTracker = reviewPromptTracker

    // Restore the last successfully resolved tier so that paying users
    // retain feature access immediately, even before the live resolution
    // completes (or when it fails on simulator).
    if let cached = self.tierCache.string(forKey: Self.tierCacheKey),
      let tier = SubscriptionTier(rawValue: cached) {
      subscriptionTier = tier
    }

    // Fast path: a returning user who already completed onboarding on this
    // device skips the loading screen and lands straight in the app. Account
    // state stays authoritative — `determineOnboarding()` reconfirms against
    // the server and can still surface the wizard for a zero-zone account.
    if onboardingRepository.isOnboardingComplete {
      onboardingPresentation = .notRequired
    }
  }

  public func makeLoginViewModel() -> LoginViewModel {
    LoginViewModel(authService: authService)
  }

  public func makeMapViewModel() -> MapViewModel {
    let viewModel = MapViewModel(
      repository: repository,
      watchZoneRepository: watchZoneRepository,
      savedApplicationRepository: savedApplicationRepository
    )
    // Open the full detail card from the summary sheet's "View full details"
    // button. Uses the synchronous overload — we already hold the full
    // `PlanningApplication`, so the sheet presents instantly with no re-fetch.
    viewModel.onShowApplicationDetail = { [weak self] application in
      self?.showApplicationDetail(application)
    }
    return viewModel
  }

  public func makeApplicationListViewModel(
    zone: WatchZone
  ) -> ApplicationListViewModel {
    let viewModel: ApplicationListViewModel
    if let offlineRepository {
      viewModel = ApplicationListViewModel(
        offlineRepository: offlineRepository,
        zone: zone,
        notificationStateRepository: notificationStateRepository
      )
    } else {
      viewModel = ApplicationListViewModel(
        repository: repository,
        zone: zone,
        notificationStateRepository: notificationStateRepository
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
        offlineRepository: offlineRepository,
        notificationStateRepository: notificationStateRepository
      )
    } else {
      viewModel = ApplicationListViewModel(
        watchZoneRepository: watchZoneRepository,
        repository: repository,
        notificationStateRepository: notificationStateRepository
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

  /// Factory for the subscription paywall presented as a sheet when
  /// `isSubscriptionPresented` is true (e.g. after tapping "View Plans" in a
  /// watch-zone upsell).
  public func makeSubscriptionViewModel() -> SubscriptionViewModel {
    SubscriptionViewModel(
      subscriptionService: subscriptionService,
      authenticationService: authService
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
    // Review-prompt value moments (GH #628): a portal tap-through and a save are
    // both genuine engagement peaks. The save callback fires only on a
    // successful false→true save (the view model guarantees this).
    viewModel.onOpenPortal = { [weak self] _ in
      self?.reviewPromptTracker?.record(.tappedPortal)
    }
    viewModel.onSaved = { [weak self] in
      self?.reviewPromptTracker?.record(.savedApplication)
    }
    return viewModel
  }

  // MARK: - Subscription Tier Resolution

  /// Resolves the subscription tier via the shared ``SubscriptionTierResolver``
  /// so this coordinator and ``SettingsViewModel`` cannot drift apart again
  /// (third recurrence after tc-aza5; original bug tc-exg6).
  public func resolveSubscriptionTier() async {
    let session = await authService.currentSession()
    let previousTier = subscriptionTier
    let result = await tierResolver.resolve(
      jwtTier: session?.subscriptionTier ?? .free,
      previousTier: subscriptionTier,
      userSub: session?.userProfile.userId
    )
    subscriptionTier = result.tier
    tierCache.set(result.tier.rawValue, forKey: Self.tierCacheKey)

    // Review-prompt upgrade signal (GH #628): a latched free→paid nudge weighted
    // below the threshold, so paying alone can never trigger the ask.
    if previousTier == .free, result.tier > .free {
      reviewPromptTracker?.record(.upgraded)
    }
    // Keep an in-flight onboarding wizard's tier live so the radius step can
    // unlock the paid range the moment a purchase resolves, without rebuilding
    // the wizard and losing the user's in-progress postcode (tc-w3cb.1/.3).
    onboardingViewModel?.subscriptionTier = result.tier

    // Post-purchase push prompt (issue #624, Prong 1): a freshly upgraded user
    // is now paying for instant alerts. If they have never been asked
    // (`.notDetermined`), trigger the system prompt at this peak-intent moment.
    // The `.notDetermined` gate self-limits this to one prompt — once the user
    // responds, the status leaves `.notDetermined` and never returns. A
    // `.denied` user is intentionally NOT re-prompted (iOS cannot re-show the
    // dialog); Prong 2's home banner deep-links them to iOS Settings instead.
    // Fired into a stored `Task` so tests can await it deterministically.
    if result.tier > .free,
      await notificationService.authorizationStatus() == .notDetermined {
      // Never stack the review prompt on the post-purchase push prompt (GH #628).
      reviewPromptTracker?.suppressThisSession()
      pendingPostPurchasePermissionPrompt = Task { [weak self] in
        _ = try? await self?.notificationService.requestPermission()
      }
    }
  }

  /// Test-only synchronisation: await the most recent post-purchase
  /// push-permission prompt. Replaces flaky `Task.sleep` waits.
  public func waitForPendingPostPurchasePrompt() async {
    await pendingPostPurchasePermissionPrompt?.value
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
}
