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
    let subscriptionService = StoreKitSubscriptionService()
    let appVersionProvider = BundleAppVersionProvider()
    let apiBaseURL = APIEnvironment.current.baseURL
    let versionConfigService = APIVersionConfigService(baseURL: apiBaseURL)
    let apiClient = URLSessionAPIClient(baseURL: apiBaseURL, authService: authService)
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
      savedApplicationRepository: savedApplicationRepository
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
      .task {
        await coordinator.resolveSubscriptionTier()
        await forceUpdateViewModel.checkVersion()
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
    }
  }

  private var mainTabView: some View {
    TabView {
      // 1. Applications
      NavigationStack {
        ApplicationListView(viewModel: coordinator.makeApplicationListViewModel())
          .id(coordinator.subscriptionTier)
          .settingsToolbar { coordinator.showSettings() }
      }
      .sheet(item: $coordinator.detailApplication) { application in
        NavigationStack {
          ApplicationDetailView(
            viewModel: coordinator.makeApplicationDetailViewModel(
              application: application
            )
          )
        }
      }
      .tabItem {
        Label("Applications", systemImage: "doc.text.magnifyingglass")
      }

      // 2. Saved
      NavigationStack {
        SavedApplicationListView(
          viewModel: coordinator.makeSavedApplicationListViewModel()
        )
        .id(coordinator.subscriptionTier)
        .settingsToolbar { coordinator.showSettings() }
      }
      .sheet(item: $coordinator.detailApplication) { application in
        NavigationStack {
          ApplicationDetailView(
            viewModel: coordinator.makeApplicationDetailViewModel(
              application: application
            )
          )
        }
      }
      .tabItem {
        Label("Saved", systemImage: "bookmark.fill")
      }

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
    }
    .tint(Color.tcAmber)
    .sheet(isPresented: $coordinator.isSettingsPresented) {
      settingsSheet
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
          coordinator.showSystemNotificationSettings()
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
      if let url = URL(string: UIApplication.openSettingsURLString) {
        openURL(url)
      }
      coordinator.isOpeningSystemNotificationSettings = false
    }
  }
}
