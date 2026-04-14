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
  @Published var selectedStatusFilter: ApplicationStatus? {
    didSet {
      if selectedStatusFilter != nil {
        isSavedFilterActive = false
      }
    }
  }
  @Published private(set) var isSavedFilterActive = false
  @Published private(set) var savedApplicationUids: Set<String> = []

  @Published private(set) var centreLat: Double = 51.5074
  @Published private(set) var centreLon: Double = -0.1278
  @Published private(set) var radiusMetres: Double = 2000

  private let repository: PlanningApplicationRepository
  private let watchZoneRepository: WatchZoneRepository
  private let savedApplicationRepository: SavedApplicationRepository?
  private let tier: SubscriptionTier
  private var applications: [PlanningApplication] = []
  private let userDefaults: UserDefaults
  private let zoneSelectionKey: String

  public var canFilter: Bool {
    tier != .free
  }

  public var canSave: Bool {
    savedApplicationRepository != nil
  }

  public var filteredAnnotations: [MapAnnotationItem] {
    if isSavedFilterActive {
      return annotations.filter { savedApplicationUids.contains($0.applicationId.value) }
    }
    guard canFilter, let filter = selectedStatusFilter else {
      return annotations
    }
    return annotations.filter { $0.status == filter }
  }

  public var isEmpty: Bool {
    hasLoaded && filteredAnnotations.isEmpty && error == nil && !isLoading
  }

  /// Whether the currently selected application is in the user's saved set.
  public var isSelectedApplicationSaved: Bool {
    guard let selected = selectedApplication else { return false }
    return savedApplicationUids.contains(selected.id.value)
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
    tier: SubscriptionTier = .free,
    userDefaults: UserDefaults = .standard,
    zoneSelectionKey: String = "lastSelectedZone.map",
    savedApplicationRepository: SavedApplicationRepository? = nil
  ) {
    self.repository = repository
    self.watchZoneRepository = watchZoneRepository
    self.savedApplicationRepository = savedApplicationRepository
    self.tier = tier
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

  public func activateSavedFilter() async {
    guard let repository = savedApplicationRepository else { return }
    selectedStatusFilter = nil
    isSavedFilterActive = true
    do {
      let saved = try await repository.loadAll()
      savedApplicationUids = Set(saved.map(\.applicationUid))
    } catch {
      savedApplicationUids = []
    }
  }

  public func deactivateSavedFilter() {
    isSavedFilterActive = false
  }

  /// Loads the set of saved application UIDs so `isSelectedApplicationSaved` can be checked.
  /// Does not activate the saved filter. No-op if no repository was provided.
  public func loadSavedStateForSelectedApplication() async {
    guard let repository = savedApplicationRepository else { return }
    do {
      let saved = try await repository.loadAll()
      savedApplicationUids = Set(saved.map(\.applicationUid))
    } catch {
      savedApplicationUids = []
    }
  }

  public func selectZone(_ zone: WatchZone) async {
    selectedZone = zone
    selectedStatusFilter = nil
    isSavedFilterActive = false
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

  /// Toggles the saved state of the currently selected application.
  /// No-op if no application is selected or no repository was provided.
  public func toggleSaveSelectedApplication() async {
    guard let repository = savedApplicationRepository,
          let selected = selectedApplication else { return }

    let uid = selected.id.value

    if savedApplicationUids.contains(uid) {
      do {
        try await repository.remove(applicationUid: uid)
        savedApplicationUids.remove(uid)
      } catch {
        // Preserve current state on failure
      }
    } else {
      do {
        try await repository.save(applicationUid: uid)
        savedApplicationUids.insert(uid)
      } catch {
        // Preserve current state on failure
      }
    }
  }
}
