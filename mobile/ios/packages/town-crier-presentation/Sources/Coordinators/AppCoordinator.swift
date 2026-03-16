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

    public func makeLegalDocumentViewModel(
        _ documentType: LegalDocumentType
    ) -> LegalDocumentViewModel {
        LegalDocumentViewModel(documentType: documentType)
    }

    public func makeMapViewModel(watchZone: WatchZone) -> MapViewModel {
        let viewModel = MapViewModel(repository: repository, watchZone: watchZone)
        viewModel.onApplicationSelected = { [weak self] id in
            self?.showApplicationDetail(id)
        }
        return viewModel
    }

    private func showApplicationDetail(_ id: PlanningApplicationId) {
        // Navigation to detail screen will be handled by tc-e5r.3
    }
}
