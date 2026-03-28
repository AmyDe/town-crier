import Foundation
import Testing
import TownCrierData
import TownCrierDomain

@testable import TownCrierPresentation

/// Validates that the app's composition root wires up without crashing.
/// Mirrors the dependency graph in TownCrierApp.init() using real concrete types,
/// catching init-time crashes (missing plists, force-unwraps, missing entitlements)
/// that only surface at runtime in the simulator.
@Suite("Composition Root")
@MainActor
struct CompositionRootTests {

  @Test func allConcreteDependenciesInitialise() {
    let authService = Auth0AuthenticationService(config: makeTestAuth0Config())
    let subscriptionService = StoreKitSubscriptionService()
    let appVersionProvider = BundleAppVersionProvider()
    // swiftlint:disable:next force_unwrapping
    let apiBaseURL = URL(string: "https://api.towncrierapp.uk")!
    let versionConfigService = APIVersionConfigService(baseURL: apiBaseURL)
    let onboardingRepository = UserDefaultsOnboardingRepository()
    let apiClient = URLSessionAPIClient(baseURL: apiBaseURL, authService: authService)
    let repository = APIPlanningApplicationRepository(apiClient: apiClient)
    let geocoder = APIPostcodeGeocoder(apiClient: apiClient)
    let notificationService = CompositeNotificationService(
      permissionProvider: SpyNotificationPermissionProvider(),
      apiService: APINotificationService(apiClient: apiClient)
    )

    let coordinator = AppCoordinator(
      repository: repository,
      authService: authService,
      subscriptionService: subscriptionService,
      geocoder: geocoder,
      watchZoneRepository: APIWatchZoneRepository(apiClient: apiClient),
      onboardingRepository: onboardingRepository,
      notificationService: notificationService,
      appVersionProvider: appVersionProvider,
      versionConfigService: versionConfigService
    )

    #expect(coordinator.detailApplication == nil)
  }

  @Test func coordinatorCreatesLoginViewModel() {
    let coordinator = makeCoordinator()
    let loginViewModel = coordinator.makeLoginViewModel()

    #expect(!loginViewModel.isAuthenticated)
  }

  @Test func coordinatorCreatesForceUpdateViewModel() {
    let coordinator = makeCoordinator()
    let forceUpdateViewModel = coordinator.makeForceUpdateViewModel()

    #expect(!forceUpdateViewModel.requiresUpdate)
  }

  @Test func coordinatorReportsOnboardingStateFromConcreteRepository() {
    let suiteName = "test-onboarding-\(UUID().uuidString)"
    // swiftlint:disable:next force_unwrapping
    let defaults = UserDefaults(suiteName: suiteName)!
    defer { UserDefaults.standard.removePersistentDomain(forName: suiteName) }

    defaults.set(true, forKey: "isOnboardingComplete")
    let onboardingRepo = UserDefaultsOnboardingRepository(defaults: defaults)

    let authService = Auth0AuthenticationService(config: makeTestAuth0Config())
    // swiftlint:disable:next force_unwrapping
    let apiBaseURL = URL(string: "https://api.towncrierapp.uk")!
    let apiClient = URLSessionAPIClient(baseURL: apiBaseURL, authService: authService)

    let coordinator = AppCoordinator(
      repository: APIPlanningApplicationRepository(apiClient: apiClient),
      authService: authService,
      subscriptionService: StoreKitSubscriptionService(),
      geocoder: APIPostcodeGeocoder(apiClient: apiClient),
      watchZoneRepository: APIWatchZoneRepository(apiClient: apiClient),
      onboardingRepository: onboardingRepo,
      notificationService: CompositeNotificationService(
        permissionProvider: SpyNotificationPermissionProvider(),
        apiService: APINotificationService(apiClient: apiClient)
      ),
      appVersionProvider: BundleAppVersionProvider(),
      versionConfigService: APIVersionConfigService(baseURL: apiBaseURL)
    )

    #expect(coordinator.isOnboardingComplete)
  }

  @Test func offlineAwareRepositoryWiresWithConcreteTypes() throws {
    let authService = Auth0AuthenticationService(config: makeTestAuth0Config())
    let apiBaseURL = try #require(URL(string: "https://api.towncrierapp.uk"))
    let apiClient = URLSessionAPIClient(baseURL: apiBaseURL, authService: authService)
    let repository = APIPlanningApplicationRepository(apiClient: apiClient)
    let connectivity = NWPathConnectivityMonitor()
    let cache = InMemoryApplicationCacheStore()
    let offlineRepository = OfflineAwareRepository(
      remote: repository,
      cache: cache,
      connectivity: connectivity
    )

    let coordinator = AppCoordinator(
      repository: repository,
      authService: authService,
      subscriptionService: StoreKitSubscriptionService(),
      offlineRepository: offlineRepository,
      geocoder: APIPostcodeGeocoder(apiClient: apiClient),
      watchZoneRepository: APIWatchZoneRepository(apiClient: apiClient),
      onboardingRepository: UserDefaultsOnboardingRepository(),
      notificationService: CompositeNotificationService(
        permissionProvider: SpyNotificationPermissionProvider(),
        apiService: APINotificationService(apiClient: apiClient)
      ),
      appVersionProvider: BundleAppVersionProvider(),
      versionConfigService: APIVersionConfigService(baseURL: apiBaseURL)
    )

    #expect(coordinator.detailApplication == nil)
  }

  @Test func metricKitCrashReporterInitialises() {
    let reporter = MetricKitCrashReporter()
    reporter.start()
  }

  @Test func auth0ConfigStoresAudience() {
    let config = makeTestAuth0Config()

    #expect(config.audience == "https://api-test.example.com")
  }

  // MARK: - Helpers

  private func makeTestAuth0Config() -> Auth0Config {
    Auth0Config(
      clientId: "test-client-id",
      domain: "test.uk.auth0.com",
      audience: "https://api-test.example.com"
    )
  }

  private func makeCoordinator() -> AppCoordinator {
    let authService = Auth0AuthenticationService(config: makeTestAuth0Config())
    // swiftlint:disable:next force_unwrapping
    let apiBaseURL = URL(string: "https://api.towncrierapp.uk")!
    let apiClient = URLSessionAPIClient(baseURL: apiBaseURL, authService: authService)
    return AppCoordinator(
      repository: APIPlanningApplicationRepository(apiClient: apiClient),
      authService: authService,
      subscriptionService: StoreKitSubscriptionService(),
      geocoder: APIPostcodeGeocoder(apiClient: apiClient),
      watchZoneRepository: APIWatchZoneRepository(apiClient: apiClient),
      onboardingRepository: UserDefaultsOnboardingRepository(),
      notificationService: CompositeNotificationService(
        permissionProvider: SpyNotificationPermissionProvider(),
        apiService: APINotificationService(apiClient: apiClient)
      ),
      appVersionProvider: BundleAppVersionProvider(),
      versionConfigService: APIVersionConfigService(baseURL: apiBaseURL)
    )
  }
}
