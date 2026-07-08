import StoreKit
import SwiftUI
import TownCrierData
import TownCrierDomain
import TownCrierPresentation
import UIKit
import UserNotifications

@main
struct TownCrierApp: App {
  @UIApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
  @Environment(\.scenePhase) private var scenePhase
  @StateObject private var coordinator: AppCoordinator
  @StateObject private var loginViewModel: LoginViewModel
  @StateObject private var forceUpdateViewModel: ForceUpdateViewModel
  @StateObject private var settingsViewModel: SettingsViewModel
  @StateObject private var anonymousBrowseCoordinator: AnonymousBrowseCoordinator
  // Single live source of truth for the appearance preference (GH#878): the
  // ONE `AppearanceStore` instance shared by `SettingsViewModel`, the
  // anonymous welcome screen's appearance control, and the root
  // `.preferredColorScheme` below — a second object merely writing the same
  // UserDefaults key would persist but not live-update this scheme until
  // relaunch.
  @StateObject private var appearanceStore: AppearanceStore
  private let crashReporter: CrashReporter
  private let notificationDelegate: NotificationDelegate
  private let pushRegistrar: PushNotificationRegistrar

  init() {
    let auth0Config = Auth0Config(
      clientId: "a9O67fPgvXtqiWqwowhYjK0tvHF4hCMZ",
      domain: "towncrierapp.uk.auth0.com",
      audience: APIEnvironment.current.baseURL.absoluteString
    )

    // Owned once here (GH#878) — every consumer below (`AppCoordinator`,
    // `AnonymousBrowseCoordinator`, and this struct's own `appearanceStore`
    // StateObject) shares this exact instance.
    let sharedAppearanceStore = AppearanceStore()
    _appearanceStore = StateObject(wrappedValue: sharedAppearanceStore)

    let authService = Auth0AuthenticationService(config: auth0Config)
    let appVersionProvider = BundleAppVersionProvider()
    let apiBaseURL = APIEnvironment.current.baseURL
    let versionConfigService = APIVersionConfigService(baseURL: apiBaseURL)
    let apiClient = URLSessionAPIClient(baseURL: apiBaseURL, authService: authService)
    // StoreKit reports verified purchases to POST /v1/subscriptions/verify so
    // the backend updates the Cosmos entitlement state (ADR 0010).
    let subscriptionService = StoreKitSubscriptionService(
      verificationService: HttpSubscriptionVerificationService(apiClient: apiClient)
    )
    let repository = APIPlanningApplicationRepository(apiClient: apiClient)
    let onboardingRepository = UserDefaultsOnboardingRepository()
    let notificationService = CompositeNotificationService(
      permissionProvider: UNNotificationPermissionProvider(),
      apiService: APINotificationService(apiClient: apiClient),
      remoteRegistrar: UIApplicationRemoteRegistrar()
    )
    let connectivityMonitor = NWPathConnectivityMonitor()
    let cacheStore = InMemoryApplicationCacheStore()
    let offlineRepository = OfflineAwareRepository(
      remote: repository,
      cache: cacheStore,
      connectivity: connectivityMonitor
    )

    let userProfileRepository = APIUserProfileRepository(apiClient: apiClient)
    let authorityRepository = APIApplicationAuthorityRepository(apiClient: apiClient)
    let watchZoneRepository = APIWatchZoneRepository(apiClient: apiClient)
    let geocoder = APIPostcodeGeocoder(apiClient: apiClient)

    // Anonymous browse mode (GH#868 Phase 3): a fresh install can reach a live
    // map of nearby applications before creating an account. No Auth0 session
    // anywhere in this path — client-side postcode lookup (never our own
    // /v1/geocode) and a parallel unauthenticated API client hitting the
    // public near-point endpoint. Deliberately parallel to the authed
    // apiClient/geocoder above rather than sharing them.
    let anonymousBrowseStateRepository = UserDefaultsAnonymousBrowseStateRepository()
    let anonymousGeocoder = PostcodesIOGeocoder()
    let anonymousApiClient = AnonymousURLSessionAPIClient(baseURL: apiBaseURL)
    let anonymousApplicationsRepository = APIAnonymousApplicationsRepository(
      apiClient: anonymousApiClient)
    // Device-local zones (GH#879 Phase 4): up to 3 on-device areas, migrated
    // from `anonymousBrowseStateRepository` on first load — see
    // `UserDefaultsDeviceLocalZoneRepository`'s own docs.
    let deviceLocalZoneRepository = UserDefaultsDeviceLocalZoneRepository(
      legacyStateRepository: anonymousBrowseStateRepository)
    // Anonymous full detail + share-link fix (GH#879 Phase 2): backs both a
    // signed-out share Universal Link and the anonymous map/summary sheet's
    // "View full details".
    let anonymousApplicationDetailRepository = APIAnonymousApplicationDetailRepository(
      apiClient: anonymousApiClient)

    let savedApplicationRepository = APISavedApplicationRepository(apiClient: apiClient)
    let notificationStateRepository = APINotificationStateRepository(apiClient: apiClient)
    // Offer-code redemption — POSTs to /v1/offer-codes/redeem (ADR 0022).
    // Injecting the service unhides the "Redeem Offer Code" row in Settings.
    let offerCodeService = HttpOfferCodeService(apiClient: apiClient)

    // App Store review prompt (GH #628): device-local gating only, no server/PII.
    let reviewRequester = CoordinatorReviewRequester()
    let reviewPromptTracker = ReviewPromptTracker(
      store: UserDefaultsReviewPromptStore(),
      requester: reviewRequester
    )
    let appCoordinator = AppCoordinator(
      repository: repository,
      authService: authService,
      subscriptionService: subscriptionService,
      userProfileRepository: userProfileRepository,
      offlineRepository: offlineRepository,
      authorityRepository: authorityRepository,
      watchZoneRepository: watchZoneRepository,
      geocoder: geocoder,
      onboardingRepository: onboardingRepository,
      notificationService: notificationService,
      appVersionProvider: appVersionProvider,
      versionConfigService: versionConfigService,
      savedApplicationRepository: savedApplicationRepository,
      offerCodeService: offerCodeService,
      notificationStateRepository: notificationStateRepository,
      badgeSetter: UIApplicationBadgeSetter(),
      reviewPromptTracker: reviewPromptTracker,
      anonymousBrowseStateRepository: anonymousBrowseStateRepository,
      appearanceStore: sharedAppearanceStore,
      anonymousApplicationDetailRepository: anonymousApplicationDetailRepository,
      deviceLocalZoneRepository: deviceLocalZoneRepository
    )
    reviewRequester.coordinator = appCoordinator  // weak; coordinator owns the tracker
    _coordinator = StateObject(wrappedValue: appCoordinator)

    let registrar = PushNotificationRegistrar(
      notificationService: notificationService,
      authService: authService
    )
    pushRegistrar = registrar

    let loginVM = appCoordinator.makeLoginViewModel()
    loginVM.onAuthenticated = {
      Task { await registrar.flushPendingRegistration() }
    }
    _loginViewModel = StateObject(wrappedValue: loginVM)

    // Anonymous browse mode (GH#868 Phase 3): both "I already have an
    // account" on the welcome screen and the map's CTA banner funnel into the
    // same single sign-up/sign-in entry point the rest of the app uses —
    // Auth0's hosted Universal Login handles both in one flow. The anonymous
    // detail screen's sign-up CTA (GH#879 Phase 2) uses the same entry point.
    appCoordinator.onRequestSignUp = {
      Task { @MainActor in await loginVM.login() }
    }
    let anonymousCoordinator = AnonymousBrowseCoordinator(
      geocoder: anonymousGeocoder,
      stateRepository: anonymousBrowseStateRepository,
      applicationsRepository: anonymousApplicationsRepository,
      deviceLocalZoneRepository: deviceLocalZoneRepository,
      appearanceStore: sharedAppearanceStore,
      appVersionProvider: appVersionProvider
    )
    anonymousCoordinator.onRequestSignIn = {
      Task { @MainActor in await loginVM.login() }
    }
    // "View full details" on the anonymous map/summary sheet presents the
    // shared root detail sheet in anonymous mode (GH#879 Phase 2) — no
    // network call, the anonymous map already holds the full application.
    anonymousCoordinator.onShowApplicationDetail = { [weak appCoordinator] application in
      appCoordinator?.showAnonymousApplicationDetail(application)
    }
    // The anonymous Settings tab's Legal/Rate-the-App rows (GH#879 Phase 3)
    // reuse the always-present `AppCoordinator`'s existing mechanisms rather
    // than duplicating them: legal documents load from a bundled JSON
    // resource with no network/auth dependency, and "Rate the App" only ever
    // opens a static App Store URL — both are safe to share verbatim between
    // the authed and anonymous surfaces.
    anonymousCoordinator.onShowPrivacyPolicy = { [weak appCoordinator] in
      appCoordinator?.showPrivacyPolicy()
    }
    anonymousCoordinator.onShowTermsOfService = { [weak appCoordinator] in
      appCoordinator?.showTermsOfService()
    }
    anonymousCoordinator.onRateApp = { [weak appCoordinator] in
      appCoordinator?.rateApp()
    }
    _anonymousBrowseCoordinator = StateObject(wrappedValue: anonymousCoordinator)

    _forceUpdateViewModel = StateObject(
      wrappedValue: appCoordinator.makeForceUpdateViewModel()
    )

    let settingsVM = appCoordinator.makeSettingsViewModel()
    settingsVM.onLogout = {
      Task { @MainActor in
        await loginVM.logout()
        // Sign-out is a deliberate return to zero state (GH#868 Phase 3.6):
        // the user lands back on the welcome screen, never the anonymous map.
        anonymousCoordinator.reset()
      }
    }
    _settingsViewModel = StateObject(wrappedValue: settingsVM)

    let delegate = NotificationDelegate(coordinator: appCoordinator)
    notificationDelegate = delegate
    UNUserNotificationCenter.current().delegate = delegate

    let reporter = MetricKitCrashReporter()
    reporter.start()
    crashReporter = reporter

    // Wire the push registrar into the AppDelegate so the UIKit lifecycle
    // callbacks (didRegisterForRemoteNotificationsWithDeviceToken /
    // didFailToRegisterForRemoteNotificationsWithError) can forward to it.
    appDelegate.registrar = registrar
    // Mirrors the registrar wiring above: lets the AppDelegate's Universal
    // Link fallback forward to the coordinator (tc-28x2, GH #763 Problem 1).
    appDelegate.coordinator = appCoordinator
  }

  var body: some Scene {
    WindowGroup {
      Group {
        if forceUpdateViewModel.requiresUpdate {
          ForceUpdateView(
            appStoreURL: URL(string: "https://apps.apple.com/app/town-crier/id000000000")
          )
        } else if loginViewModel.isAuthenticated {
          authenticatedRootView
        } else {
          // Three-state root routing (GH#868 Phase 3): an authenticated
          // session always wins outright (above); otherwise
          // AnonymousBrowseCoordinator itself resolves the remaining two
          // states — persisted anonymous browse state routes straight to the
          // tab shell, anything else starts at the welcome screen. `LoginView`
          // still exists and is reached from there via "I already have an
          // account", which calls `loginViewModel.login()` directly (Auth0's
          // hosted Universal Login), the same single entry point sign-up and
          // sign-in both use.
          //
          // The legal-document sheet and App Store review modifier
          // (GH#879 Phase 3) mount here too — mutually exclusive with
          // `MainTabView`'s own copies inside `authenticatedRootView` above,
          // since only one branch of this `Group` is ever in the tree at a
          // time, so `coordinator`'s flags can never be observed twice.
          AnonymousBrowseView(coordinator: anonymousBrowseCoordinator)
            .sheet(item: $coordinator.presentedLegalDocument) { documentType in
              NavigationStack {
                LegalDocumentView(viewModel: LegalDocumentViewModel(documentType: documentType))
              }
            }
            .openingAppStoreReview(when: coordinator)
        }
      }
      // Session-restore on cold launch: previously lived in `LoginView`'s own
      // `.task`, relocated here now that view is no longer unconditionally in
      // the tree. Runs once; a restored session flips `isAuthenticated` and
      // the switch above re-renders into `authenticatedRootView`.
      .task {
        await loginViewModel.checkExistingSession()
      }
      // Primary inbound UL + Auth0-callback entry point — OpenURLModifier.swift.
      // `.handlingUniversalLinks` below is a belt-and-braces fallback —
      // UniversalLinkModifier.swift (tc-28x2, GH #763 Problem 1).
      .handlingOpenURL(coordinator: coordinator)
      .handlingUniversalLinks(coordinator: coordinator)
      // Keyed on auth state (not a bare `.task`) so profile-ensure RE-RUNS on
      // the unauthenticated->authenticated transition. On a fresh install the
      // launch-time task fires while still signed out (no token -> the
      // idempotent POST /v1/me can't create the Cosmos UserProfile); without an
      // `id:` it never re-runs when the user signs in, so the profile is missing
      // for the whole session and the first watch-zone POST 500s on its quota
      // check (Cosmos 404). Re-running on the transition ensures the profile the
      // moment a session exists. `checkVersion()` is idempotent, so re-running
      // it here is harmless (tc-k9fk).
      .task(id: loginViewModel.isAuthenticated) {
        await coordinator.resolveSubscriptionTier()
        // Resolve onboarding only once authenticated, and strictly after
        // resolveSubscriptionTier() has ensured the server profile above — the
        // wizard's first watch-zone save needs that profile or it 500s
        // (tc-k9fk / tc-w3cb.1).
        if loginViewModel.isAuthenticated {
          await coordinator.determineOnboarding()
        }
        await forceUpdateViewModel.checkVersion()
      }
      .onChange(of: scenePhase) { oldPhase, newPhase in
        // Reconcile the app icon badge with the server-side unread watermark on
        // every foreground entry — we pull rather than rely on silent push so
        // cross-device read-state changes propagate without server-side fan-out
        // (docs/specs/notifications-unread-watermark.md#ios-badge-foreground-push).
        guard newPhase == .active else { return }
        // Loyalty review signal: only a background→active re-entry counts (#628).
        coordinator.recordAppForegrounded(isReactivation: oldPhase == .background)
        Task { await coordinator.syncBadge() }
      }
      .requestingReview(when: coordinator)
      .preferredColorScheme(appearanceStore.appearanceMode.preferredColorScheme)
      .alert(
        coordinator.deepLinkError?.userTitle ?? "Error",
        isPresented: Binding(
          get: { coordinator.deepLinkError != nil },
          set: { if !$0 { coordinator.deepLinkError = nil } }
        )
      ) {
        Button("OK", role: .cancel) {}
      } message: {
        if let error = coordinator.deepLinkError {
          Text(error.userMessage)
        }
      }
      // Single sheet modifier observing `coordinator.detailApplication`.
      // Hoisted from per-tab NavigationStacks so a deep link arriving
      // while the app is on Saved/Map/Zones still presents the detail
      // sheet — previously two sibling `.sheet(item:)` modifiers raced
      // and only the active tab's sheet would fire (tc-dt3x).
      .sheet(item: $coordinator.detailApplication) { application in
        NavigationStack {
          ApplicationDetailView(
            viewModel: coordinator.makeApplicationDetailViewModel(
              application: application
            )
          )
        }
      }
    }
  }

  /// Routes the authenticated user to the first-run onboarding wizard, a
  /// neutral loading screen, or the main app — depending on whether the
  /// coordinator has determined onboarding is needed (tc-w3cb.1). The wizard
  /// presents only once profile-ensure has run, so its first watch-zone save
  /// cannot 500.
  @ViewBuilder
  private var authenticatedRootView: some View {
    switch coordinator.onboardingPresentation {
    case .required:
      OnboardingView(viewModel: coordinator.makeOnboardingViewModel())
    case .notRequired:
      MainTabView(coordinator: coordinator, settingsViewModel: settingsViewModel)
    case .undetermined:
      onboardingLoadingView
    }
  }

  /// Branded loading screen shown while onboarding need is still resolving.
  /// Keeps the wizard from flashing before the watch-zone load completes.
  private var onboardingLoadingView: some View {
    ZStack {
      Color.tcBackground.ignoresSafeArea()
      ProgressView()
        .tint(Color.tcAmber)
    }
  }
}
