import SwiftUI
import TownCrierData
import TownCrierDomain
import TownCrierPresentation

@main
struct TownCrierApp: App {
    @StateObject private var coordinator: AppCoordinator
    @StateObject private var loginViewModel: LoginViewModel
    @StateObject private var forceUpdateViewModel: ForceUpdateViewModel
    private let crashReporter: CrashReporter

    init() {
        let repository = InMemoryPlanningApplicationRepository()
        let authService = Auth0AuthenticationService()
        let subscriptionService = StoreKitSubscriptionService()
        let appVersionProvider = BundleAppVersionProvider()
        let versionConfigService = StubVersionConfigService()

        let appCoordinator = AppCoordinator(
            repository: repository,
            authService: authService,
            subscriptionService: subscriptionService,
            appVersionProvider: appVersionProvider,
            versionConfigService: versionConfigService
        )
        _coordinator = StateObject(wrappedValue: appCoordinator)
        _loginViewModel = StateObject(
            wrappedValue: appCoordinator.makeLoginViewModel()
        )
        _forceUpdateViewModel = StateObject(
            wrappedValue: appCoordinator.makeForceUpdateViewModel()
        )

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
                    HomeView(viewModel: coordinator.makeHomeViewModel())
                } else {
                    LoginView(viewModel: loginViewModel)
                }
            }
            .task {
                await forceUpdateViewModel.checkVersion()
            }
        }
    }
}
