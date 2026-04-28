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
    let notificationService = CompositeNotificationService(
      permissionProvider: SpyNotificationPermissionProvider(),
      apiService: APINotificationService(apiClient: apiClient),
      remoteRegistrar: SpyRemoteNotificationRegistering()
    )

    let userProfileRepository = APIUserProfileRepository(apiClient: apiClient)

    let watchZoneRepository = APIWatchZoneRepository(apiClient: apiClient)
    let coordinator = AppCoordinator(
      repository: repository,
      authService: authService,
      subscriptionService: subscriptionService,
      userProfileRepository: userProfileRepository,
      watchZoneRepository: watchZoneRepository,
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
      userProfileRepository: APIUserProfileRepository(apiClient: apiClient),
      watchZoneRepository: APIWatchZoneRepository(apiClient: apiClient),
      onboardingRepository: onboardingRepo,
      notificationService: CompositeNotificationService(
        permissionProvider: SpyNotificationPermissionProvider(),
        apiService: APINotificationService(apiClient: apiClient),
        remoteRegistrar: SpyRemoteNotificationRegistering()
      ),
      appVersionProvider: BundleAppVersionProvider(),
      versionConfigService: APIVersionConfigService(baseURL: apiBaseURL)
    )

    #expect(coordinator.isOnboardingComplete)
  }

  @Test func metricKitCrashReporterInitialises() {
    let reporter = MetricKitCrashReporter()
    reporter.start()
  }

  @Test func auth0ConfigStoresAudience() {
    let config = makeTestAuth0Config()

    #expect(config.audience == "https://api-test.example.com")
  }

  @Test func coordinatorCreatesWatchZoneListViewModel() {
    let coordinator = makeCoordinator()
    let vm = coordinator.makeWatchZoneListViewModel()

    #expect(vm.zones.isEmpty)
    #expect(!vm.isLoading)
  }

  @Test func coordinatorCreatesWatchZoneEditorViewModel() {
    let coordinator = makeCoordinatorWithGeocoder()
    let vm = coordinator.makeWatchZoneEditorViewModel()

    #expect(!vm.isEditing)
  }

  @Test func coordinatorCreatesMapViewModelWithZoneRepository() async {
    let authService = Auth0AuthenticationService(config: makeTestAuth0Config())
    // swiftlint:disable:next force_unwrapping
    let apiBaseURL = URL(string: "https://api.towncrierapp.uk")!
    let apiClient = URLSessionAPIClient(baseURL: apiBaseURL, authService: authService)

    let watchZoneRepository = APIWatchZoneRepository(apiClient: apiClient)
    let coordinator = AppCoordinator(
      repository: APIPlanningApplicationRepository(apiClient: apiClient),
      authService: authService,
      subscriptionService: StoreKitSubscriptionService(),
      userProfileRepository: APIUserProfileRepository(apiClient: apiClient),
      watchZoneRepository: watchZoneRepository,
      onboardingRepository: UserDefaultsOnboardingRepository(),
      notificationService: CompositeNotificationService(
        permissionProvider: SpyNotificationPermissionProvider(),
        apiService: APINotificationService(apiClient: apiClient),
        remoteRegistrar: SpyRemoteNotificationRegistering()
      ),
      appVersionProvider: BundleAppVersionProvider(),
      versionConfigService: APIVersionConfigService(baseURL: apiBaseURL)
    )

    let vm = coordinator.makeMapViewModel()
    #expect(vm.annotations.isEmpty)
  }

  @Test func coordinatorCreatesApplicationListViewModelWithPlaceholderZone() async {
    let authService = Auth0AuthenticationService(config: makeTestAuth0Config())
    // swiftlint:disable:next force_unwrapping
    let apiBaseURL = URL(string: "https://api.towncrierapp.uk")!
    let apiClient = URLSessionAPIClient(baseURL: apiBaseURL, authService: authService)
    let authorityRepository = APIApplicationAuthorityRepository(apiClient: apiClient)

    let coordinator = AppCoordinator(
      repository: APIPlanningApplicationRepository(apiClient: apiClient),
      authService: authService,
      subscriptionService: StoreKitSubscriptionService(),
      userProfileRepository: APIUserProfileRepository(apiClient: apiClient),
      authorityRepository: authorityRepository,
      watchZoneRepository: APIWatchZoneRepository(apiClient: apiClient),
      onboardingRepository: UserDefaultsOnboardingRepository(),
      notificationService: CompositeNotificationService(
        permissionProvider: SpyNotificationPermissionProvider(),
        apiService: APINotificationService(apiClient: apiClient),
        remoteRegistrar: SpyRemoteNotificationRegistering()
      ),
      appVersionProvider: BundleAppVersionProvider(),
      versionConfigService: APIVersionConfigService(baseURL: apiBaseURL)
    )

    let vm = coordinator.makeApplicationListViewModel()
    #expect(vm.filteredApplications.isEmpty)
    #expect(vm.error == nil)
  }

  @Test func coordinatorCreatesDetailViewModelWithSavedRepository() {
    let coordinator = makeCoordinatorWithSavedRepository()
    let vm = coordinator.makeApplicationDetailViewModel(application: .pendingReview)

    #expect(vm.canSave)
  }

  @Test func coordinatorCreatesListViewModelWithSavedRepository() {
    let coordinator = makeCoordinatorWithSavedRepository()
    let vm = coordinator.makeApplicationListViewModel(zone: .cambridge)

    #expect(vm.canSave)
  }

  @Test func coordinatorCreatesMapViewModelWithSavedRepository() {
    let coordinator = makeCoordinatorWithSavedRepository()
    let vm = coordinator.makeMapViewModel()

    #expect(vm.canSave)
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
      userProfileRepository: APIUserProfileRepository(apiClient: apiClient),
      watchZoneRepository: APIWatchZoneRepository(apiClient: apiClient),
      onboardingRepository: UserDefaultsOnboardingRepository(),
      notificationService: CompositeNotificationService(
        permissionProvider: SpyNotificationPermissionProvider(),
        apiService: APINotificationService(apiClient: apiClient),
        remoteRegistrar: SpyRemoteNotificationRegistering()
      ),
      appVersionProvider: BundleAppVersionProvider(),
      versionConfigService: APIVersionConfigService(baseURL: apiBaseURL)
    )
  }

  private func makeCoordinatorWithSavedRepository() -> AppCoordinator {
    let authService = Auth0AuthenticationService(config: makeTestAuth0Config())
    // swiftlint:disable:next force_unwrapping
    let apiBaseURL = URL(string: "https://api.towncrierapp.uk")!
    let apiClient = URLSessionAPIClient(baseURL: apiBaseURL, authService: authService)
    return AppCoordinator(
      repository: APIPlanningApplicationRepository(apiClient: apiClient),
      authService: authService,
      subscriptionService: StoreKitSubscriptionService(),
      userProfileRepository: APIUserProfileRepository(apiClient: apiClient),
      watchZoneRepository: APIWatchZoneRepository(apiClient: apiClient),
      onboardingRepository: UserDefaultsOnboardingRepository(),
      notificationService: CompositeNotificationService(
        permissionProvider: SpyNotificationPermissionProvider(),
        apiService: APINotificationService(apiClient: apiClient),
        remoteRegistrar: SpyRemoteNotificationRegistering()
      ),
      appVersionProvider: BundleAppVersionProvider(),
      versionConfigService: APIVersionConfigService(baseURL: apiBaseURL),
      savedApplicationRepository: APISavedApplicationRepository(apiClient: apiClient)
    )
  }

  private func makeCoordinatorWithGeocoder() -> AppCoordinator {
    let authService = Auth0AuthenticationService(config: makeTestAuth0Config())
    // swiftlint:disable:next force_unwrapping
    let apiBaseURL = URL(string: "https://api.towncrierapp.uk")!
    let apiClient = URLSessionAPIClient(baseURL: apiBaseURL, authService: authService)
    return AppCoordinator(
      repository: APIPlanningApplicationRepository(apiClient: apiClient),
      authService: authService,
      subscriptionService: StoreKitSubscriptionService(),
      userProfileRepository: APIUserProfileRepository(apiClient: apiClient),
      watchZoneRepository: APIWatchZoneRepository(apiClient: apiClient),
      geocoder: APIPostcodeGeocoder(apiClient: apiClient),
      onboardingRepository: UserDefaultsOnboardingRepository(),
      notificationService: CompositeNotificationService(
        permissionProvider: SpyNotificationPermissionProvider(),
        apiService: APINotificationService(apiClient: apiClient),
        remoteRegistrar: SpyRemoteNotificationRegistering()
      ),
      appVersionProvider: BundleAppVersionProvider(),
      versionConfigService: APIVersionConfigService(baseURL: apiBaseURL)
    )
  }
}
