import SwiftUI
import TownCrierData
import TownCrierPresentation

@main
struct TownCrierApp: App {
    @StateObject private var coordinator: AppCoordinator

    init() {
        let repository = InMemoryPlanningApplicationRepository()
        _coordinator = StateObject(wrappedValue: AppCoordinator(repository: repository))
    }

    var body: some Scene {
        WindowGroup {
            HomeView(viewModel: coordinator.makeHomeViewModel())
        }
    }
}
