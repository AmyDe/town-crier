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
    private let crashReporter: CrashReporter
    private let notificationDelegate: NotificationDelegate

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
                    HomeView(viewModel: coordinator.makeHomeViewModel())
                } else {
                    LoginView(viewModel: loginViewModel)
                }
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
}
