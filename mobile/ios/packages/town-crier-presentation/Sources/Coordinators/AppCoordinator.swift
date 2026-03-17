import Combine
import TownCrierDomain

/// Root coordinator managing top-level navigation.
@MainActor
public final class AppCoordinator: ObservableObject {
    @Published public var detailApplication: PlanningApplication?
    @Published public var deepLinkError: DomainError?

    public var isOnboardingComplete: Bool {
        onboardingRepository?.isOnboardingComplete ?? false
    }

    private let repository: PlanningApplicationRepository
    private let authService: AuthenticationService
    private let subscriptionService: SubscriptionService
    private let geocoder: PostcodeGeocoder?
    private let watchZoneRepository: WatchZoneRepository?
    private let onboardingRepository: OnboardingRepository?
    private let notificationService: NotificationService?
    private let appVersionProvider: AppVersionProvider?
    private let versionConfigService: VersionConfigService?

    public init(
        repository: PlanningApplicationRepository,
        authService: AuthenticationService,
        subscriptionService: SubscriptionService,
        geocoder: PostcodeGeocoder? = nil,
        watchZoneRepository: WatchZoneRepository? = nil,
        onboardingRepository: OnboardingRepository? = nil,
        notificationService: NotificationService? = nil,
        appVersionProvider: AppVersionProvider? = nil,
        versionConfigService: VersionConfigService? = nil
    ) {
        self.repository = repository
        self.authService = authService
        self.subscriptionService = subscriptionService
        self.geocoder = geocoder
        self.watchZoneRepository = watchZoneRepository
        self.onboardingRepository = onboardingRepository
        self.notificationService = notificationService
        self.appVersionProvider = appVersionProvider
        self.versionConfigService = versionConfigService
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

    public func makeOnboardingViewModel() -> OnboardingViewModel {
        guard let geocoder = geocoder, let watchZoneRepository = watchZoneRepository,
              let onboardingRepository = onboardingRepository,
              let notificationService = notificationService
        else {
            preconditionFailure("Onboarding dependencies not provided to AppCoordinator")
        }
        let viewModel = OnboardingViewModel(
            geocoder: geocoder,
            watchZoneRepository: watchZoneRepository,
            onboardingRepository: onboardingRepository,
            notificationService: notificationService
        )
        return viewModel
    }

    public func makeSubscriptionViewModel() -> SubscriptionViewModel {
        SubscriptionViewModel(subscriptionService: subscriptionService)
    }

    public func makeSettingsViewModel() -> SettingsViewModel {
        guard let appVersionProvider = appVersionProvider else {
            preconditionFailure("AppVersionProvider not provided to AppCoordinator")
        }
        let viewModel = SettingsViewModel(
            authService: authService,
            subscriptionService: subscriptionService,
            appVersionProvider: appVersionProvider
        )
        return viewModel
    }

    public func makeForceUpdateViewModel() -> ForceUpdateViewModel {
        guard let appVersionProvider = appVersionProvider,
              let versionConfigService = versionConfigService
        else {
            preconditionFailure("Version dependencies not provided to AppCoordinator")
        }
        return ForceUpdateViewModel(
            versionConfigService: versionConfigService,
            appVersionProvider: appVersionProvider
        )
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

    public func handleDeepLink(_ deepLink: DeepLink) {
        deepLinkError = nil
        switch deepLink {
        case .applicationDetail(let id):
            showApplicationDetail(id)
        }
    }

    private func showApplicationDetail(_ id: PlanningApplicationId) {
        Task {
            do {
                detailApplication = try await repository.fetchApplication(by: id)
            } catch let domainError as DomainError {
                deepLinkError = domainError
            } catch {
                deepLinkError = .unexpected(error.localizedDescription)
            }
        }
    }
}
