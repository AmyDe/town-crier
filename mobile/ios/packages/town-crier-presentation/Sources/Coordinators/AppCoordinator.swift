import Combine
import TownCrierDomain
import os

/// Root coordinator managing top-level navigation.
@MainActor
public final class AppCoordinator: ObservableObject {
  private static let logger = Logger(subsystem: "uk.towncrierapp", category: "AppCoordinator")

  @Published public var detailApplication: PlanningApplication?
  @Published public var deepLinkError: DomainError?
  @Published public var presentedLegalDocument: LegalDocumentType?
  @Published public var isManageSubscriptionPresented = false
  @Published public var isSubscriptionPresented = false
  @Published public var isAddingWatchZone = false
  @Published public var editingWatchZone: WatchZone?
  @Published public private(set) var subscriptionTier: SubscriptionTier = .free

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
  private let watchZoneRepository: WatchZoneRepository
  private let geocoder: PostcodeGeocoder?
  private let appVersionProvider: AppVersionProvider
  private let versionConfigService: VersionConfigService

  public init(
    repository: PlanningApplicationRepository,
    authService: AuthenticationService,
    subscriptionService: SubscriptionService,
    userProfileRepository: UserProfileRepository,
    offlineRepository: OfflineAwareRepository? = nil,
    authorityRepository: ApplicationAuthorityRepository? = nil,
    watchZoneRepository: WatchZoneRepository,
    geocoder: PostcodeGeocoder? = nil,
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
    self.watchZoneRepository = watchZoneRepository
    self.geocoder = geocoder
    self.onboardingRepository = onboardingRepository
    self.notificationService = notificationService
    self.appVersionProvider = appVersionProvider
    self.versionConfigService = versionConfigService
  }

  public func makeLoginViewModel() -> LoginViewModel {
    LoginViewModel(authService: authService)
  }

  public func makeMapViewModel() -> MapViewModel {
    MapViewModel(repository: repository, watchZoneRepository: watchZoneRepository)
  }

  public func makeApplicationListViewModel(
    zone: WatchZone
  ) -> ApplicationListViewModel {
    let viewModel: ApplicationListViewModel
    if let offlineRepository {
      viewModel = ApplicationListViewModel(
        offlineRepository: offlineRepository, zone: zone, tier: subscriptionTier)
    } else {
      viewModel = ApplicationListViewModel(
        repository: repository, zone: zone, tier: subscriptionTier)
    }
    viewModel.onApplicationSelected = { [weak self] id in
      self?.showApplicationDetail(id)
    }
    return viewModel
  }

  /// Creates a list view model that resolves the user's first watch zone at load time.
  /// Callers should prefer `makeApplicationListViewModel(zone:)` when a specific zone is known.
  public func makeApplicationListViewModel() -> ApplicationListViewModel {
    let viewModel: ApplicationListViewModel
    if let offlineRepository {
      viewModel = ApplicationListViewModel(
        watchZoneRepository: watchZoneRepository,
        offlineRepository: offlineRepository,
        tier: subscriptionTier
      )
    } else {
      viewModel = ApplicationListViewModel(
        watchZoneRepository: watchZoneRepository,
        repository: repository,
        tier: subscriptionTier
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

  // MARK: - Subscription Tier Resolution

  /// Resolves the subscription tier by consulting the JWT session, StoreKit,
  /// and the server profile, then picks the highest tier. Mirrors the same
  /// triple-source resolution used by ``SettingsViewModel``.
  public func resolveSubscriptionTier() async {
    var jwtTier: SubscriptionTier = .free
    if let session = await authService.currentSession() {
      jwtTier = session.subscriptionTier
    }

    let serverTier = await fetchServerTier()
    let storeKitTier = await subscriptionService.currentEntitlement()?.tier ?? .free
    subscriptionTier = max(serverTier, max(storeKitTier, jwtTier))
  }

  private func fetchServerTier() async -> SubscriptionTier {
    do {
      if let profile = try await userProfileRepository.fetch() {
        return profile.tier
      }
      return .free
    } catch {
      Self.logger.error(
        "Failed to fetch server profile for subscription tier: \(error.localizedDescription)"
      )
      return .free
    }
  }

  // MARK: - Watch Zone Factories

  public func makeWatchZoneListViewModel() -> WatchZoneListViewModel {
    let viewModel = WatchZoneListViewModel(
      repository: watchZoneRepository,
      featureGate: FeatureGate(tier: subscriptionTier)
    )
    viewModel.onAddZone = { [weak self] in
      self?.isAddingWatchZone = true
    }
    viewModel.onEditZone = { [weak self] zone in
      self?.editingWatchZone = zone
    }
    viewModel.onViewPlans = { [weak self] in
      self?.isSubscriptionPresented = true
    }
    return viewModel
  }

  public func makeWatchZoneEditorViewModel(
    editing zone: WatchZone? = nil
  ) -> WatchZoneEditorViewModel {
    guard let geocoder else {
      fatalError("PostcodeGeocoder must be injected to create WatchZoneEditorViewModel")
    }
    let viewModel = WatchZoneEditorViewModel(
      geocoder: geocoder,
      repository: watchZoneRepository,
      tier: subscriptionTier,
      editing: zone
    )
    viewModel.onSave = { [weak self] _ in
      self?.isAddingWatchZone = false
      self?.editingWatchZone = nil
    }
    return viewModel
  }

  // MARK: - Settings Navigation

  public func showPrivacyPolicy() {
    presentedLegalDocument = .privacyPolicy
  }

  public func showTermsOfService() {
    presentedLegalDocument = .termsOfService
  }

  public func showManageSubscription() {
    isManageSubscriptionPresented = true
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
