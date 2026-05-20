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
  @Environment(\.openURL) private var openURL
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
      notificationStateRepository: notificationStateRepository,
      badgeSetter: UIApplicationBadgeSetter()
    )
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
  }

  var body: some Scene {
    WindowGroup {
      Group {
        if forceUpdateViewModel.requiresUpdate {
          ForceUpdateView(
            appStoreURL: URL(string: "https://apps.apple.com/app/town-crier/id000000000")
          )
        } else if loginViewModel.isAuthenticated {
          mainTabView
        } else {
          LoginView(viewModel: loginViewModel)
        }
      }
      .onOpenURL { url in
        AuthCallbackHandler.handle(url: url)
      }
      .onContinueUserActivity(NSUserActivityTypeBrowsingWeb) { activity in
        guard let url = activity.webpageURL,
          let deepLink = UniversalLinkParser.parse(url)
        else { return }
        coordinator.handleDeepLink(deepLink)
      }
      .task {
        await coordinator.resolveSubscriptionTier()
        await forceUpdateViewModel.checkVersion()
      }
      .onChange(of: scenePhase) { _, newPhase in
        // Reconcile the app icon badge with the server-side unread watermark
        // whenever the scene becomes active — both on first launch and on
        // every foreground entry. We deliberately pull rather than rely on
        // silent push so cross-device read-state changes propagate without
        // server-side push fan-out (spec
        // docs/specs/notifications-unread-watermark.md#ios-badge-foreground-push).
        guard newPhase == .active else { return }
        Task { await coordinator.syncBadge() }
      }
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

  private var mainTabView: some View {
    TabView(selection: $coordinator.selectedTab) {
      // 1. Applications
      NavigationStack {
        ApplicationListView(viewModel: coordinator.makeApplicationListViewModel())
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
    .sheet(isPresented: $coordinator.isSubscriptionPresented) {
      NavigationStack {
        SubscriptionView(viewModel: coordinator.makeSubscriptionViewModel())
      }
    }
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
        }
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
    #if os(iOS)
      .manageSubscriptionsSheet(
        isPresented: $coordinator.isManageSubscriptionPresented.dispatchingSetOnMain()
      )
    #endif
    .onChange(of: coordinator.isOpeningSystemNotificationSettings) { _, requested in
      guard requested else { return }
      if let url = URL(string: AppCoordinator.systemNotificationSettingsURLString) {
        openURL(url)
      }
      coordinator.isOpeningSystemNotificationSettings = false
    }
  }
}
