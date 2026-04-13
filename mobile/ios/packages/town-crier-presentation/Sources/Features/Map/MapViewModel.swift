import Combine
import TownCrierDomain

/// ViewModel driving the map view with planning application pins.
@MainActor
public final class MapViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published private(set) var annotations: [MapAnnotationItem] = []
  @Published private(set) var isLoading = false
  @Published var error: DomainError?
  @Published private(set) var selectedApplication: PlanningApplication?
  @Published private(set) var hasLoaded = false

  @Published private(set) var centreLat: Double = 51.5074
  @Published private(set) var centreLon: Double = -0.1278
  @Published private(set) var radiusMetres: Double = 2000

  private let repository: PlanningApplicationRepository?
  private let offlineRepository: OfflineAwareRepository?
  private let authorityRepository: ApplicationAuthorityRepository?
  private let applicationRepository: PlanningApplicationRepository?
  private let watchZoneRepository: WatchZoneRepository
  private var applications: [PlanningApplication] = []

  public var isEmpty: Bool {
    hasLoaded && annotations.isEmpty && error == nil && !isLoading
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

  var onApplicationSelected: ((PlanningApplicationId) -> Void)?

  public init(repository: PlanningApplicationRepository, watchZoneRepository: WatchZoneRepository) {
    self.repository = repository
    self.offlineRepository = nil
    self.authorityRepository = nil
    self.applicationRepository = nil
    self.watchZoneRepository = watchZoneRepository
  }

  public init(offlineRepository: OfflineAwareRepository, watchZoneRepository: WatchZoneRepository) {
    self.repository = nil
    self.offlineRepository = offlineRepository
    self.authorityRepository = nil
    self.applicationRepository = nil
    self.watchZoneRepository = watchZoneRepository
  }

  public init(
    authorityRepository: ApplicationAuthorityRepository,
    applicationRepository: PlanningApplicationRepository,
    watchZoneRepository: WatchZoneRepository
  ) {
    self.repository = nil
    self.offlineRepository = nil
    self.authorityRepository = authorityRepository
    self.applicationRepository = applicationRepository
    self.watchZoneRepository = watchZoneRepository
  }

  public func loadApplications() async {
    isLoading = true
    error = nil
    do {
      if let zone = try? await watchZoneRepository.loadAll().first {
        centreLat = zone.centre.latitude
        centreLon = zone.centre.longitude
        radiusMetres = zone.radiusMetres
      }

      let fetched: [PlanningApplication]
      if let authorityRepository, let applicationRepository {
        fetched = try await fetchViaAuthorities(
          authorityRepository: authorityRepository,
          applicationRepository: applicationRepository
        )
      } else if let offlineRepository {
        let entry = try await offlineRepository.fetchApplications(
          for: LocalAuthority(code: "", name: ""))
        fetched = entry.data
      } else if let repository {
        fetched = try await repository.fetchApplications(for: LocalAuthority(code: "", name: ""))
      } else {
        fetched = []
      }
      applications = fetched
      annotations = fetched.compactMap { app in
        guard let location = app.location else { return nil }
        return MapAnnotationItem(application: app, coordinate: location)
      }
    } catch {
      handleError(error)
    }
    isLoading = false
    hasLoaded = true
  }

  private func fetchViaAuthorities(
    authorityRepository: ApplicationAuthorityRepository,
    applicationRepository: PlanningApplicationRepository
  ) async throws -> [PlanningApplication] {
    let result = try await authorityRepository.fetchAuthorities()
    var allApplications: [PlanningApplication] = []
    for authority in result.authorities {
      do {
        let apps = try await applicationRepository.fetchApplications(for: authority)
        allApplications.append(contentsOf: apps)
      } catch {
        // Partial failure: skip this authority, continue with others
        continue
      }
    }
    return allApplications
  }

  public func selectApplication(_ id: PlanningApplicationId) {
    selectedApplication = applications.first { $0.id == id }
    onApplicationSelected?(id)
  }

  public func clearSelection() {
    selectedApplication = nil
  }
}
