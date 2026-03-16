import Combine
import TownCrierDomain

/// Root coordinator managing top-level navigation.
@MainActor
public final class AppCoordinator: ObservableObject {
    private let repository: PlanningApplicationRepository

    public init(repository: PlanningApplicationRepository) {
        self.repository = repository
    }

    public func makeHomeViewModel() -> HomeViewModel {
        HomeViewModel()
    }
}
