import Combine
import TownCrierDomain

/// Root coordinator managing top-level navigation.
@MainActor
public final class AppCoordinator: ObservableObject {
  @Published public var detailApplication: PlanningApplication?
  @Published public var deepLinkError: DomainError?

  public var isOnboardingComplete: Bool {
    onboardingRepository.isOnboardingComplete
  }

  private let repository: PlanningApplicationRepository
  private let authService: AuthenticationService
  private let subscriptionService: SubscriptionService
  private let userProfileRepository: UserProfileRepository
  private let onboardingRepository: OnboardingRepository
  private let notificationService: NotificationService
  private let offlineRepository: OfflineAwareRepository?
  private let authorityRepository: ApplicationAuthorityRepository?
  private let appVersionProvider: AppVersionProvider
  private let versionConfigService: VersionConfigService

  public init(
    repository: PlanningApplicationRepository,
    authService: AuthenticationService,
    subscriptionService: SubscriptionService,
    userProfileRepository: UserProfileRepository,
    offlineRepository: OfflineAwareRepository? = nil,
    authorityRepository: ApplicationAuthorityRepository? = nil,
    onboardingRepository: OnboardingRepository,
    notificationService: NotificationService,
    appVersionProvider: AppVersionProvider,
    versionConfigService: VersionConfigService
  ) {
    self.repository = repository
    self.authService = authService
    self.subscriptionService = subscriptionService
    self.userProfileRepository = userProfileRepository
    self.offlineRepository = offlineRepository
    self.authorityRepository = authorityRepository
    self.onboardingRepository = onboardingRepository
    self.notificationService = notificationService
    self.appVersionProvider = appVersionProvider
    self.versionConfigService = versionConfigService
  }

  public func makeLoginViewModel() -> LoginViewModel {
    LoginViewModel(authService: authService)
  }

  public func makeMapViewModel(watchZone: WatchZone) -> MapViewModel {
    if let authorityRepository {
      return MapViewModel(
        authorityRepository: authorityRepository,
        applicationRepository: repository,
        watchZone: watchZone
      )
    }
    if let offlineRepository {
      return MapViewModel(offlineRepository: offlineRepository, watchZone: watchZone)
    }
    return MapViewModel(repository: repository, watchZone: watchZone)
  }

  public func makeApplicationListViewModel(
    authority: LocalAuthority
  ) -> ApplicationListViewModel {
    let viewModel: ApplicationListViewModel
    if let offlineRepository {
      viewModel = ApplicationListViewModel(
        offlineRepository: offlineRepository, authority: authority)
    } else {
      viewModel = ApplicationListViewModel(repository: repository, authority: authority)
    }
    viewModel.onApplicationSelected = { [weak self] id in
      self?.showApplicationDetail(id)
    }
    return viewModel
  }

  public func makeApplicationListViewModel() -> ApplicationListViewModel {
    let viewModel: ApplicationListViewModel
    if let authorityRepository {
      viewModel = ApplicationListViewModel(
        authorityRepository: authorityRepository,
        applicationRepository: repository
      )
    } else {
      viewModel = makeApplicationListViewModel(
        authority: LocalAuthority(code: "", name: "")
      )
    }
    viewModel.onApplicationSelected = { [weak self] id in
      self?.showApplicationDetail(id)
    }
    return viewModel
  }

  public func makeSettingsViewModel() -> SettingsViewModel {
    SettingsViewModel(
      authService: authService,
      subscriptionService: subscriptionService,
      userProfileRepository: userProfileRepository,
      appVersionProvider: appVersionProvider,
      notificationService: notificationService
    )
  }

  public func makeForceUpdateViewModel() -> ForceUpdateViewModel {
    ForceUpdateViewModel(
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
