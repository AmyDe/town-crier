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
  private let crashReporter: CrashReporter
  private let notificationDelegate: NotificationDelegate
  private let pushRegistrar: PushNotificationRegistrar

  init() {
    let auth0Config = Auth0Config(
      clientId: "a9O67fPgvXtqiWqwowhYjK0tvHF4hCMZ",
      domain: "towncrierapp.uk.auth0.com",
      audience: APIEnvironment.current.baseURL.absoluteString
    )

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
      reviewPromptTracker: reviewPromptTracker
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

    _forceUpdateViewModel = StateObject(
      wrappedValue: appCoordinator.makeForceUpdateViewModel()
    )

    let settingsVM = appCoordinator.makeSettingsViewModel()
    settingsVM.onLogout = {
      Task { @MainActor in
        await loginVM.logout()
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
          LoginView(viewModel: loginViewModel)
        }
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
      .preferredColorScheme(settingsViewModel.appearanceMode.preferredColorScheme)
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
      mainTabView
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

  private var mainTabView: some View {
    TabView(selection: $coordinator.selectedTab) {
      // 1. Applications
      NavigationStack {
        VStack(spacing: 0) {
          // Paid-user push-permission nudge (issue #624, Prong 2). Hidden
          // unless the user is on a paid tier and notifications are not
          // authorized. The `.id` is hoisted onto the VStack so both the
          // banner and the list rebuild when the resolved tier changes (e.g.
          // straight after a purchase).
          PushNudgeBanner(viewModel: coordinator.makePushNudgeViewModel())
          ApplicationListView(viewModel: coordinator.makeApplicationListViewModel())
        }
        .id(coordinator.subscriptionTier)
        .settingsToolbar { coordinator.showSettings() }
      }
      .tabItem {
        Label("Applications", systemImage: "doc.text.magnifyingglass")
      }
      .tag(MainTab.applications)

      // 2. Saved
      NavigationStack {
        SavedApplicationListView(
          viewModel: coordinator.makeSavedApplicationListViewModel()
        )
        .id(coordinator.subscriptionTier)
        .settingsToolbar { coordinator.showSettings() }
      }
      .tabItem {
        Label("Saved", systemImage: "bookmark.fill")
      }
      .tag(MainTab.saved)

      // 3. Map
      NavigationStack {
        MapView(viewModel: coordinator.makeMapViewModel())
          .id(coordinator.subscriptionTier)
          .navigationTitle("Map")
          #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
          #endif
          .settingsToolbar { coordinator.showSettings() }
      }
      .tabItem {
        Label("Map", systemImage: "map")
      }
      .tag(MainTab.map)

      // 4. Zones
      NavigationStack {
        WatchZoneListView(viewModel: coordinator.makeWatchZoneListViewModel())
          .id(coordinator.subscriptionTier)
          .settingsToolbar { coordinator.showSettings() }
      }
      .sheet(isPresented: $coordinator.isAddingWatchZone) {
        WatchZoneEditorView(
          viewModel: coordinator.makeWatchZoneEditorViewModel()
        )
      }
      .sheet(item: $coordinator.editingWatchZone) { zone in
        WatchZoneEditorView(
          viewModel: coordinator.makeWatchZoneEditorViewModel(editing: zone)
        )
      }
      .tabItem {
        Label("Zones", systemImage: "mappin.and.ellipse")
      }
      .tag(MainTab.zones)
    }
    .tint(Color.tcAmber)
    .sheet(isPresented: $coordinator.isSettingsPresented) {
      settingsSheet
    }
    // Subscription paywall — presented when an upsell (e.g. "View Plans" in
    // the watch-zone quota banner) sets `isSubscriptionPresented`. Hoisted to
    // the TabView so the paywall reaches the user regardless of active tab.
    // On dismiss, re-resolve the tier so a successful purchase unlocks gated
    // features (e.g. the larger watch-zone radius) live, without an app
    // relaunch — the tier-keyed views rebuild on the change (tc-w3cb.3).
    .sheet(
      isPresented: $coordinator.isSubscriptionPresented,
      onDismiss: { Task { await coordinator.resolveSubscriptionTier() } },
      content: {
        NavigationStack {
          SubscriptionView(viewModel: coordinator.makeSubscriptionViewModel())
        }
      }
    )
  }

  /// Settings sheet — presented from the gear icon installed on every tab.
  /// Hosts the existing SettingsView and the legal/notification/manage-sub
  /// side-effects that were previously bound to the Settings tab.
  @ViewBuilder
  private var settingsSheet: some View {
    NavigationStack {
      SettingsView(
        viewModel: settingsViewModel,
        onNotificationPreferences: {
          coordinator.showNotificationPreferences()
        },
        onManageSubscription: {
          coordinator.showManageSubscription()
        },
        onPrivacyPolicy: {
          coordinator.showPrivacyPolicy()
        },
        onTermsOfService: {
          coordinator.showTermsOfService()
        },
        // Surfacing the "Redeem Offer Code" row only when an OfferCodeService
        // was injected — SettingsView hides the row when this callback is nil.
        onRedeemOfferCode: coordinator.isOfferCodeRedemptionAvailable
          ? { coordinator.showRedeemOfferCode() }
          : nil,
        onRateApp: coordinator.rateApp
      )
      .navigationDestination(isPresented: $coordinator.isNotificationPreferencesPresented) {
        NotificationPreferencesView(
          viewModel: coordinator.makeNotificationPreferencesViewModel(),
          onZonesTap: {
            coordinator.isSettingsPresented = false
            coordinator.selectedTab = .zones
          },
          onSystemSettingsTap: {
            coordinator.showSystemNotificationSettings()
          }
        )
      }
    }
    .sheet(item: $coordinator.presentedLegalDocument) { documentType in
      NavigationStack {
        LegalDocumentView(viewModel: LegalDocumentViewModel(documentType: documentType))
      }
    }
    // Offer-code redemption — presented from the "Redeem Offer Code" row in
    // Settings (ADR 0022). The factory returns nil if no OfferCodeService was
    // injected, in which case the row is also hidden, so the sheet body is a
    // no-op fallback that should never render.
    .sheet(isPresented: $coordinator.isRedeemOfferCodePresented) {
      NavigationStack {
        if let viewModel = coordinator.makeRedeemOfferCodeViewModel() {
          RedeemOfferCodeView(viewModel: viewModel)
        }
      }
    }
    #if os(iOS)
      .manageSubscriptionsSheet(
        isPresented: $coordinator.isManageSubscriptionPresented.dispatchingSetOnMain()
      )
    #endif
    // App-layer edge for coordinator-driven deep links: open the URL and reset
    // the flag. Extracted into a shared modifier (mirrors ReviewPromptRequest-
    // Modifier) so the file stays UIKit-free and within length limits.
    .openingSystemNotificationSettings(when: coordinator)
    .openingAppStoreReview(when: coordinator)
  }
}
