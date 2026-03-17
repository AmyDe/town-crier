import Combine
import TownCrierDomain

/// Root coordinator managing top-level navigation.
@MainActor
public final class AppCoordinator: ObservableObject {
    @Published public var detailApplication: PlanningApplication?

    private let repository: PlanningApplicationRepository
    private let authService: AuthenticationService
    private let subscriptionService: SubscriptionService

    public init(
        repository: PlanningApplicationRepository,
        authService: AuthenticationService,
        subscriptionService: SubscriptionService
    ) {
        self.repository = repository
        self.authService = authService
        self.subscriptionService = subscriptionService
    }

    public func makeLoginViewModel() -> LoginViewModel {
        LoginViewModel(authService: authService)
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

    public func makeApplicationListViewModel(
        authority: LocalAuthority
    ) -> ApplicationListViewModel {
        let viewModel = ApplicationListViewModel(repository: repository, authority: authority)
        viewModel.onApplicationSelected = { [weak self] id in
            self?.showApplicationDetail(id)
        }
        return viewModel
    }

    public func makeSubscriptionViewModel() -> SubscriptionViewModel {
        SubscriptionViewModel(subscriptionService: subscriptionService)
    }

    public func makeApplicationDetailViewModel(
        application: PlanningApplication
    ) -> ApplicationDetailViewModel {
        let viewModel = ApplicationDetailViewModel(application: application)
        viewModel.onDismiss = { [weak self] in
            self?.detailApplication = nil
        }
        return viewModel
    }

    private func showApplicationDetail(_ id: PlanningApplicationId) {
        Task {
            do {
                detailApplication = try await repository.fetchApplication(by: id)
            } catch {
                // Detail navigation failed — stay on current screen
            }
        }
    }
}
