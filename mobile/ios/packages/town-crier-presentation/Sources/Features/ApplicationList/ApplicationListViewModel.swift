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
  private let authorityRepository: ApplicationAuthorityRepository?
  private let applicationRepository: PlanningApplicationRepository?
  private let authority: LocalAuthority?
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
    authority: LocalAuthority,
    tier: SubscriptionTier = .free
  ) {
    self.repository = repository
    self.offlineRepository = nil
    self.authorityRepository = nil
    self.applicationRepository = nil
    self.authority = authority

    self.tier = tier
  }

  public init(
    offlineRepository: OfflineAwareRepository,
    authority: LocalAuthority,
    tier: SubscriptionTier = .free
  ) {
    self.repository = nil
    self.offlineRepository = offlineRepository
    self.authorityRepository = nil
    self.applicationRepository = nil
    self.authority = authority

    self.tier = tier
  }

  public init(
    authorityRepository: ApplicationAuthorityRepository,
    applicationRepository: PlanningApplicationRepository,
    tier: SubscriptionTier = .free
  ) {
    self.repository = nil
    self.offlineRepository = nil
    self.authorityRepository = authorityRepository
    self.applicationRepository = applicationRepository
    self.authority = nil

    self.tier = tier
  }

  public func loadApplications() async {
    isLoading = true
    error = nil
    do {
      let fetched: [PlanningApplication]
      if let authorityRepository, let applicationRepository {
        fetched = try await fetchViaAuthorities(
          authorityRepository: authorityRepository,
          applicationRepository: applicationRepository
        )
      } else if let authority, let offlineRepository {
        let zone = try WatchZone(
          id: WatchZoneId(authority.code),
          name: authority.name.isEmpty ? "Default" : authority.name,
          centre: Coordinate(latitude: 0, longitude: 0),
          radiusMetres: 1
        )
        let entry = try await offlineRepository.fetchApplications(for: zone)
        fetched = entry.data
      } else if let authority, let repository {
        let zone = try WatchZone(
          id: WatchZoneId(authority.code),
          name: authority.name.isEmpty ? "Default" : authority.name,
          centre: Coordinate(latitude: 0, longitude: 0),
          radiusMetres: 1
        )
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

  private func fetchViaAuthorities(
    authorityRepository: ApplicationAuthorityRepository,
    applicationRepository: PlanningApplicationRepository
  ) async throws -> [PlanningApplication] {
    let result = try await authorityRepository.fetchAuthorities()
    var allApplications: [PlanningApplication] = []
    for authority in result.authorities {
      let zone = try WatchZone(
        id: WatchZoneId(authority.code),
        name: authority.name,
        centre: Coordinate(latitude: 0, longitude: 0),
        radiusMetres: 1
      )
      let apps = try await applicationRepository.fetchApplications(for: zone)
      allApplications.append(contentsOf: apps)
    }
    return allApplications
  }

  public func selectApplication(_ id: PlanningApplicationId) {
    onApplicationSelected?(id)
  }
}
