import SwiftUI
import TownCrierData
import TownCrierDomain
import TownCrierPresentation

@main
struct TownCrierApp: App {
    @StateObject private var coordinator: AppCoordinator
    @StateObject private var loginViewModel: LoginViewModel
    private let crashReporter: CrashReporter

    init() {
        let repository = InMemoryPlanningApplicationRepository()
        let authService = Auth0AuthenticationService()
        let subscriptionService = StoreKitSubscriptionService()

        let appCoordinator = AppCoordinator(
            repository: repository,
            authService: authService,
            subscriptionService: subscriptionService
        )
        _coordinator = StateObject(wrappedValue: appCoordinator)
        _loginViewModel = StateObject(
            wrappedValue: appCoordinator.makeLoginViewModel()
        )

        let reporter = MetricKitCrashReporter()
        reporter.start()
        crashReporter = reporter
    }

    var body: some Scene {
        WindowGroup {
            if loginViewModel.isAuthenticated {
                HomeView(viewModel: coordinator.makeHomeViewModel())
            } else {
                LoginView(viewModel: loginViewModel)
            }
        }
    }
}
