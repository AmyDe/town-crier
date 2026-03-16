import SwiftUI
import TownCrierData
import TownCrierDomain
import TownCrierPresentation

@main
struct TownCrierApp: App {
    @StateObject private var coordinator: AppCoordinator
    private let crashReporter: CrashReporter

    init() {
        let repository = InMemoryPlanningApplicationRepository()
        _coordinator = StateObject(wrappedValue: AppCoordinator(repository: repository))

        let reporter = MetricKitCrashReporter()
        reporter.start()
        crashReporter = reporter
    }

    var body: some Scene {
        WindowGroup {
            HomeView(viewModel: coordinator.makeHomeViewModel())
        }
    }
}
