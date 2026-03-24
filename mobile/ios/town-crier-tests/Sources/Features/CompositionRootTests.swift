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
        let repository = InMemoryPlanningApplicationRepository()
        let auth0Config = Auth0Config(clientId: "test-client-id", domain: "test.uk.auth0.com")
        let authService = Auth0AuthenticationService(config: auth0Config)
        let subscriptionService = StoreKitSubscriptionService()
        let appVersionProvider = BundleAppVersionProvider()
        let apiBaseURL = URL(string: "https://api.towncrierapp.uk")!
        let versionConfigService = APIVersionConfigService(baseURL: apiBaseURL)
        let onboardingRepository = UserDefaultsOnboardingRepository()

        let coordinator = AppCoordinator(
            repository: repository,
            authService: authService,
            subscriptionService: subscriptionService,
            onboardingRepository: onboardingRepository,
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

    @Test func metricKitCrashReporterInitialises() {
        let reporter = MetricKitCrashReporter()
        reporter.start()
    }

    // MARK: - Helpers

    private func makeCoordinator() -> AppCoordinator {
        let auth0Config = Auth0Config(clientId: "test-client-id", domain: "test.uk.auth0.com")
        let apiBaseURL = URL(string: "https://api.towncrierapp.uk")!
        return AppCoordinator(
            repository: InMemoryPlanningApplicationRepository(),
            authService: Auth0AuthenticationService(config: auth0Config),
            subscriptionService: StoreKitSubscriptionService(),
            onboardingRepository: UserDefaultsOnboardingRepository(),
            appVersionProvider: BundleAppVersionProvider(),
            versionConfigService: APIVersionConfigService(baseURL: apiBaseURL)
        )
    }
}
