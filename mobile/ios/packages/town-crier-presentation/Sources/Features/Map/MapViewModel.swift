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

  private let repository: PlanningApplicationRepository
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
    self.watchZoneRepository = watchZoneRepository
  }

  public func loadApplications() async {
    isLoading = true
    error = nil
    do {
      let zones = try await watchZoneRepository.loadAll()
      guard let zone = zones.first else {
        isLoading = false
        hasLoaded = true
        return
      }

      centreLat = zone.centre.latitude
      centreLon = zone.centre.longitude
      radiusMetres = zone.radiusMetres

      let fetched = try await repository.fetchApplications(for: zone)
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

  public func selectApplication(_ id: PlanningApplicationId) {
    selectedApplication = applications.first { $0.id == id }
    onApplicationSelected?(id)
  }

  public func clearSelection() {
    selectedApplication = nil
  }
}
