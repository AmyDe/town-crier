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
  @Published private(set) var isAllZonesSelected = false
  @Published private(set) var isSavedFilterActive = false
  @Published private(set) var isLoadingSaved = false
  @Published private(set) var savedApplicationUids: Set<String> = []

  /// Sentinel persisted to UserDefaults to indicate the synthetic 'All' selection.
  /// Watch zone IDs are UUID strings so cannot collide with this value.
  static let allZonesSentinel = "__all__"

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
      return applications.filter { savedApplicationUids.contains($0.id.value) }
    }
    guard canFilter, let filter = selectedStatusFilter else {
      return applications
    }
    return applications.filter { $0.status == filter }
  }

  public var isEmpty: Bool {
    filteredApplications.isEmpty && error == nil && !isLoading && !isLoadingSaved
  }

  /// Identifies which copy the empty state should render. Only meaningful when
  /// `isEmpty` is true; the View renders one of these messages accordingly.
  public enum EmptyStateKind: Equatable, Sendable {
    /// 'All' selected with the Saved filter off — encourage the user to pick a
    /// zone or turn on Saved to see their bookmarks.
    case allZonesNoSavedFilter
    /// Saved filter on but the user has no bookmarks (in-zone or, when 'All',
    /// across all zones).
    case savedFilterNoResults
    /// A real watch zone is selected but it has no applications yet.
    case zoneNoApplications
  }

  public var emptyStateKind: EmptyStateKind {
    if isAllZonesSelected, !isSavedFilterActive {
      return .allZonesNoSavedFilter
    }
    if isSavedFilterActive {
      return .savedFilterNoResults
    }
    return .zoneNoApplications
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
        // Refresh any in-place-edited zone (same id, new radius/centre) so its
        // updated geometry propagates downstream. If the selection has been
        // deleted — or nothing is selected yet — fall back to
        // `resolveInitialSelection`, which honours both real zone IDs and the
        // synthetic 'All' sentinel persisted in UserDefaults.
        if isAllZonesSelected {
          selectedZone = nil
        } else if let currentId = selectedZone?.id,
                  let updated = loadedZones.first(where: { $0.id == currentId }) {
          selectedZone = updated
        } else {
          resolveInitialSelection(from: loadedZones)
        }
      }
      if isAllZonesSelected {
        // 'All' + Saved active → populate from saved payloads; otherwise empty.
        applications = []
        isLoading = false
        return
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
    isLoadingSaved = true
    do {
      let saved = try await repository.loadAll()
      savedApplicationUids = Set(saved.map(\.applicationUid))
      if isAllZonesSelected {
        // 'All' + Saved → show every saved app, regardless of zone.
        // Server-denormalised payloads on SavedApplication.application carry
        // the data; entries lacking a payload (e.g. legacy saves) are dropped.
        applications = saved.compactMap(\.application)
          .sorted { $0.receivedDate > $1.receivedDate }
      }
    } catch {
      savedApplicationUids = []
      if isAllZonesSelected {
        applications = []
      }
    }
    isLoadingSaved = false
  }

  public func deactivateSavedFilter() {
    isSavedFilterActive = false
    if isAllZonesSelected {
      // 'All' without Saved has no per-zone source — clear so the empty state
      // and discoverability prompt take over.
      applications = []
    }
  }

  public func selectZone(_ zone: WatchZone) async {
    isAllZonesSelected = false
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

  /// Selects the synthetic 'All' option. Lists are sourced from the Saved
  /// repository when the Saved filter is active; otherwise the list is empty
  /// and the view shows a discoverability prompt.
  public func selectAllZones() async {
    isAllZonesSelected = true
    selectedZone = nil
    selectedStatusFilter = nil
    userDefaults.set(Self.allZonesSentinel, forKey: zoneSelectionKey)
    if isSavedFilterActive, let repository = savedApplicationRepository {
      isLoadingSaved = true
      do {
        let saved = try await repository.loadAll()
        savedApplicationUids = Set(saved.map(\.applicationUid))
        applications = saved.compactMap(\.application)
          .sorted { $0.receivedDate > $1.receivedDate }
      } catch {
        savedApplicationUids = []
        applications = []
      }
      isLoadingSaved = false
    } else {
      applications = []
    }
  }

  public func selectApplication(_ id: PlanningApplicationId) {
    onApplicationSelected?(id)
  }

  /// Restores the previous-session selection (real zone or 'All') from
  /// UserDefaults, falling back to the first zone when nothing is persisted.
  private func resolveInitialSelection(from zones: [WatchZone]) {
    let savedId = userDefaults.string(forKey: zoneSelectionKey)
    if savedId == Self.allZonesSentinel {
      isAllZonesSelected = true
      selectedZone = nil
      return
    }
    if let savedId,
       let savedZone = zones.first(where: { $0.id.value == savedId }) {
      selectedZone = savedZone
      return
    }
    selectedZone = zones.first
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
