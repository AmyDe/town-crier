import Combine
import TownCrierDomain

/// ViewModel driving the filterable list of planning applications.
@MainActor
public final class ApplicationListViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published private(set) var applications: [PlanningApplication] = []
  @Published var selectedStatusFilter: ApplicationStatus?
  @Published private(set) var isLoading = false
  @Published var error: DomainError?

  private let repository: PlanningApplicationRepository?
  private let offlineRepository: OfflineAwareRepository?
  private let watchZoneRepository: WatchZoneRepository?
  private var zone: WatchZone?
  private let tier: SubscriptionTier

  var onApplicationSelected: ((PlanningApplicationId) -> Void)?

  public var canFilter: Bool {
    tier != .free
  }

  public var filteredApplications: [PlanningApplication] {
    guard canFilter, let filter = selectedStatusFilter else {
      return applications
    }
    return applications.filter { $0.status == filter }
  }

  public var isEmpty: Bool {
    filteredApplications.isEmpty && error == nil && !isLoading
  }

  public var isNetworkError: Bool {
    error == .networkUnavailable
  }

  public var isServerError: Bool {
    if case .serverError = error { return true }
    return false
  }

  public var isSessionExpired: Bool {
    error == .sessionExpired
  }

  public init(
    repository: PlanningApplicationRepository,
    zone: WatchZone,
    tier: SubscriptionTier = .free
  ) {
    self.repository = repository
    self.offlineRepository = nil
    self.watchZoneRepository = nil
    self.zone = zone
    self.tier = tier
  }

  public init(
    offlineRepository: OfflineAwareRepository,
    zone: WatchZone,
    tier: SubscriptionTier = .free
  ) {
    self.repository = nil
    self.offlineRepository = offlineRepository
    self.watchZoneRepository = nil
    self.zone = zone
    self.tier = tier
  }

  public init(
    watchZoneRepository: WatchZoneRepository,
    repository: PlanningApplicationRepository,
    tier: SubscriptionTier = .free
  ) {
    self.repository = repository
    self.offlineRepository = nil
    self.watchZoneRepository = watchZoneRepository
    self.zone = nil
    self.tier = tier
  }

  public init(
    watchZoneRepository: WatchZoneRepository,
    offlineRepository: OfflineAwareRepository,
    tier: SubscriptionTier = .free
  ) {
    self.repository = nil
    self.offlineRepository = offlineRepository
    self.watchZoneRepository = watchZoneRepository
    self.zone = nil
    self.tier = tier
  }

  public func loadApplications() async {
    isLoading = true
    error = nil
    do {
      if zone == nil, let watchZoneRepository {
        let zones = try await watchZoneRepository.loadAll()
        zone = zones.first
      }

      guard let zone else {
        applications = []
        isLoading = false
        return
      }

      let fetched: [PlanningApplication]
      if let offlineRepository {
        let entry = try await offlineRepository.fetchApplications(for: zone)
        fetched = entry.data
      } else if let repository {
        fetched = try await repository.fetchApplications(for: zone)
      } else {
        fetched = []
      }
      applications = fetched.sorted { $0.receivedDate > $1.receivedDate }
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  public func selectApplication(_ id: PlanningApplicationId) {
    onApplicationSelected?(id)
  }
}
