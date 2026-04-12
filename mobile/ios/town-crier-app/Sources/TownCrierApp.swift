import SwiftUI
import TownCrierData
import TownCrierDomain
import TownCrierPresentation
import UserNotifications

@main
struct TownCrierApp: App {
  @StateObject private var coordinator: AppCoordinator
  @StateObject private var loginViewModel: LoginViewModel
  @StateObject private var forceUpdateViewModel: ForceUpdateViewModel
  @StateObject private var settingsViewModel: SettingsViewModel
  @StateObject private var applicationListViewModel: ApplicationListViewModel
  @StateObject private var mapViewModel: MapViewModel
  private let crashReporter: CrashReporter
  private let notificationDelegate: NotificationDelegate

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
      apiService: APINotificationService(apiClient: apiClient)
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

    let appCoordinator = AppCoordinator(
      repository: repository,
      authService: authService,
      subscriptionService: subscriptionService,
      userProfileRepository: userProfileRepository,
      offlineRepository: offlineRepository,
      authorityRepository: authorityRepository,
      onboardingRepository: onboardingRepository,
      notificationService: notificationService,
      appVersionProvider: appVersionProvider,
      versionConfigService: versionConfigService
    )
    _coordinator = StateObject(wrappedValue: appCoordinator)

    let loginVM = appCoordinator.makeLoginViewModel()
    _loginViewModel = StateObject(wrappedValue: loginVM)

    _forceUpdateViewModel = StateObject(
      wrappedValue: appCoordinator.makeForceUpdateViewModel()
    )

    #if DEBUG
      let listVM = appCoordinator.makeApplicationListViewModel(
        authority: SampleData.camden
      )
      let mapVM = appCoordinator.makeMapViewModel(watchZone: SampleData.watchZone)
    #else
      let listVM = appCoordinator.makeApplicationListViewModel()
      // swiftlint:disable force_try
      let mapVM = appCoordinator.makeMapViewModel(
        watchZone: try! WatchZone(
          postcode: try! Postcode("SW1A 1AA"),
          centre: try! Coordinate(latitude: 51.5074, longitude: -0.1278),
          radiusMetres: 1000
        )
      )
    // swiftlint:enable force_try
    #endif

    _applicationListViewModel = StateObject(wrappedValue: listVM)
    _mapViewModel = StateObject(wrappedValue: mapVM)

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
        await forceUpdateViewModel.checkVersion()
      }
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
      NavigationStack {
        ApplicationListView(viewModel: applicationListViewModel)
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

      NavigationStack {
        MapView(viewModel: mapViewModel)
          .navigationTitle("Map")
          #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
          #endif
      }
      .tabItem {
        Label("Map", systemImage: "map")
      }

      NavigationStack {
        SettingsView(viewModel: settingsViewModel)
      }
      .tabItem {
        Label("Settings", systemImage: "gearshape")
      }
    }
    .tint(Color.tcAmber)
  }
}
