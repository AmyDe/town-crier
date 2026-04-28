import Combine
import Foundation
import TownCrierDomain

/// ViewModel driving the filterable list of planning applications.
@MainActor
public final class ApplicationListViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published private(set) var applications: [PlanningApplication] = []
  @Published var selectedStatusFilter: ApplicationStatus? {
    didSet {
      if selectedStatusFilter != nil {
        isSavedFilterActive = false
      }
    }
  }
  @Published private(set) var isLoading = false
  @Published var error: DomainError?
  @Published private(set) var zones: [WatchZone] = []
  @Published private(set) var selectedZone: WatchZone?
  @Published private(set) var isSavedFilterActive = false
  @Published private(set) var savedApplicationUids: Set<String> = []

  /// Full saved application objects from the most recent saved-filter activation.
  /// Used to merge cross-zone applications into the filtered display list.
  private var savedApplications: [SavedApplication] = []

  private let repository: PlanningApplicationRepository?
  private let offlineRepository: OfflineAwareRepository?
  private let watchZoneRepository: WatchZoneRepository?
  private let savedApplicationRepository: SavedApplicationRepository?
  private var zone: WatchZone?
  private let tier: SubscriptionTier
  private let userDefaults: UserDefaults
  private let zoneSelectionKey: String

  var onApplicationSelected: ((PlanningApplicationId) -> Void)?

  public var canFilter: Bool {
    tier != .free
  }

  public var canSave: Bool {
    savedApplicationRepository != nil
  }

  public var showZonePicker: Bool {
    zones.count > 1
  }

  public var filteredApplications: [PlanningApplication] {
    if isSavedFilterActive {
      let existingIds = Set(applications.map(\.id.value))
      let fromCurrentList = applications.filter { savedApplicationUids.contains($0.id.value) }
      let fromSavedData = savedApplications.compactMap { saved -> PlanningApplication? in
        guard let app = saved.application, !existingIds.contains(app.id.value) else { return nil }
        return app
      }
      return fromCurrentList + fromSavedData
    }
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
    tier: SubscriptionTier = .free,
    savedApplicationRepository: SavedApplicationRepository? = nil
  ) {
    self.repository = repository
    self.offlineRepository = nil
    self.watchZoneRepository = nil
    self.savedApplicationRepository = savedApplicationRepository
    self.zone = zone
    self.tier = tier
    self.userDefaults = .standard
    self.zoneSelectionKey = ""
  }

  public init(
    offlineRepository: OfflineAwareRepository,
    zone: WatchZone,
    tier: SubscriptionTier = .free,
    savedApplicationRepository: SavedApplicationRepository? = nil
  ) {
    self.repository = nil
    self.offlineRepository = offlineRepository
    self.watchZoneRepository = nil
    self.savedApplicationRepository = savedApplicationRepository
    self.zone = zone
    self.tier = tier
    self.userDefaults = .standard
    self.zoneSelectionKey = ""
  }

  public init(
    watchZoneRepository: WatchZoneRepository,
    repository: PlanningApplicationRepository,
    tier: SubscriptionTier = .free,
    userDefaults: UserDefaults = .standard,
    zoneSelectionKey: String = "lastSelectedZone.applications",
    savedApplicationRepository: SavedApplicationRepository? = nil
  ) {
    self.repository = repository
    self.offlineRepository = nil
    self.watchZoneRepository = watchZoneRepository
    self.savedApplicationRepository = savedApplicationRepository
    self.zone = nil
    self.tier = tier
    self.userDefaults = userDefaults
    self.zoneSelectionKey = zoneSelectionKey
  }

  public init(
    watchZoneRepository: WatchZoneRepository,
    offlineRepository: OfflineAwareRepository,
    tier: SubscriptionTier = .free,
    userDefaults: UserDefaults = .standard,
    zoneSelectionKey: String = "lastSelectedZone.applications",
    savedApplicationRepository: SavedApplicationRepository? = nil
  ) {
    self.repository = nil
    self.offlineRepository = offlineRepository
    self.watchZoneRepository = watchZoneRepository
    self.savedApplicationRepository = savedApplicationRepository
    self.zone = nil
    self.tier = tier
    self.userDefaults = userDefaults
    self.zoneSelectionKey = zoneSelectionKey
  }

  public func loadApplications() async {
    isLoading = true
    error = nil
    do {
      if let watchZoneRepository {
        let loadedZones = try await watchZoneRepository.loadAll()
        zones = loadedZones
        // Always refresh `selectedZone` from the reloaded list so an in-place
        // edit (same id, new radius/centre) propagates through to the list's
        // header pill and downstream filters. Falling back to
        // `resolveInitialZone` only when the id is missing (zone deleted)
        // preserves the previous-session restore behaviour.
        if let currentId = selectedZone?.id,
           let updated = loadedZones.first(where: { $0.id == currentId }) {
          selectedZone = updated
        } else {
          selectedZone = resolveInitialZone(from: loadedZones)
        }
      }
      guard let activeZone = selectedZone ?? zone else {
        applications = []
        isLoading = false
        return
      }
      applications = try await fetchApplications(for: activeZone)
        .sorted { $0.receivedDate > $1.receivedDate }
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  public func activateSavedFilter() async {
    guard let repository = savedApplicationRepository else { return }
    selectedStatusFilter = nil
    isSavedFilterActive = true
    do {
      let saved = try await repository.loadAll()
      savedApplications = saved
      savedApplicationUids = Set(saved.map(\.applicationUid))
    } catch {
      savedApplications = []
      savedApplicationUids = []
    }
  }

  public func deactivateSavedFilter() {
    isSavedFilterActive = false
    savedApplications = []
  }

  public func selectZone(_ zone: WatchZone) async {
    selectedZone = zone
    selectedStatusFilter = nil
    isSavedFilterActive = false
    userDefaults.set(zone.id.value, forKey: zoneSelectionKey)
    isLoading = true
    error = nil
    do {
      applications = try await fetchApplications(for: zone)
        .sorted { $0.receivedDate > $1.receivedDate }
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  public func selectApplication(_ id: PlanningApplicationId) {
    onApplicationSelected?(id)
  }

  private func resolveInitialZone(from zones: [WatchZone]) -> WatchZone? {
    if let savedId = userDefaults.string(forKey: zoneSelectionKey),
       let savedZone = zones.first(where: { $0.id.value == savedId }) {
      return savedZone
    }
    return zones.first
  }

  private func fetchApplications(for zone: WatchZone) async throws -> [PlanningApplication] {
    if let offlineRepository {
      return try await offlineRepository.fetchApplications(for: zone).data
    } else if let repository {
      return try await repository.fetchApplications(for: zone)
    }
    return []
  }
}
