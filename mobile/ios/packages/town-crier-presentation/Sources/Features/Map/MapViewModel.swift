import Combine
import Foundation
import TownCrierDomain

/// ViewModel driving the map view with planning application pins.
@MainActor
public final class MapViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published private(set) var annotations: [MapAnnotationItem] = []
  @Published private(set) var isLoading = false
  @Published var error: DomainError?
  @Published private(set) var selectedApplication: PlanningApplication?
  @Published private(set) var hasLoaded = false
  @Published private(set) var zones: [WatchZone] = []
  @Published private(set) var selectedZone: WatchZone?

  @Published private(set) var centreLat: Double = 51.5074
  @Published private(set) var centreLon: Double = -0.1278
  @Published private(set) var radiusMetres: Double = 2000

  private let repository: PlanningApplicationRepository
  private let watchZoneRepository: WatchZoneRepository
  private var applications: [PlanningApplication] = []
  private let userDefaults: UserDefaults
  private let zoneSelectionKey: String

  public var isEmpty: Bool {
    hasLoaded && annotations.isEmpty && error == nil && !isLoading
  }

  public var showZonePicker: Bool {
    zones.count > 1
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

  public init(
    repository: PlanningApplicationRepository,
    watchZoneRepository: WatchZoneRepository,
    userDefaults: UserDefaults = .standard,
    zoneSelectionKey: String = "lastSelectedZone.map"
  ) {
    self.repository = repository
    self.watchZoneRepository = watchZoneRepository
    self.userDefaults = userDefaults
    self.zoneSelectionKey = zoneSelectionKey
  }

  public func loadApplications() async {
    isLoading = true
    error = nil
    do {
      let loadedZones = try await watchZoneRepository.loadAll()
      zones = loadedZones
      if selectedZone == nil || !loadedZones.contains(where: { $0.id == selectedZone?.id }) {
        selectedZone = resolveInitialZone(from: loadedZones)
      }
      guard let zone = selectedZone else {
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

  public func selectZone(_ zone: WatchZone) async {
    selectedZone = zone
    userDefaults.set(zone.id.value, forKey: zoneSelectionKey)
    centreLat = zone.centre.latitude
    centreLon = zone.centre.longitude
    radiusMetres = zone.radiusMetres
    isLoading = true
    error = nil
    do {
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
  }

  private func resolveInitialZone(from zones: [WatchZone]) -> WatchZone? {
    if let savedId = userDefaults.string(forKey: zoneSelectionKey),
       let savedZone = zones.first(where: { $0.id.value == savedId }) {
      return savedZone
    }
    return zones.first
  }

  public func selectApplication(_ id: PlanningApplicationId) {
    selectedApplication = applications.first { $0.id == id }
    onApplicationSelected?(id)
  }

  public func clearSelection() {
    selectedApplication = nil
  }
}
