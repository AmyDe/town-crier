import Combine
import Foundation
import TownCrierDomain

/// ViewModel driving the filterable list of planning applications.
@MainActor
public final class ApplicationListViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published private(set) var applications: [PlanningApplication] = []
  @Published var selectedStatusFilter: ApplicationStatus?
  @Published private(set) var isLoading = false
  @Published var error: DomainError?
  @Published private(set) var zones: [WatchZone] = []
  @Published private(set) var selectedZone: WatchZone?

  private let repository: PlanningApplicationRepository?
  private let offlineRepository: OfflineAwareRepository?
  private let watchZoneRepository: WatchZoneRepository?
  private var zone: WatchZone?
  private let tier: SubscriptionTier
  private let userDefaults: UserDefaults
  private let zoneSelectionKey: String

  var onApplicationSelected: ((PlanningApplicationId) -> Void)?

  public var canFilter: Bool {
    tier != .free
  }

  public var showZonePicker: Bool {
    zones.count > 1
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
    self.userDefaults = .standard
    self.zoneSelectionKey = ""
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
    self.userDefaults = .standard
    self.zoneSelectionKey = ""
  }

  public init(
    watchZoneRepository: WatchZoneRepository,
    repository: PlanningApplicationRepository,
    tier: SubscriptionTier = .free,
    userDefaults: UserDefaults = .standard,
    zoneSelectionKey: String = "lastSelectedZone.applications"
  ) {
    self.repository = repository
    self.offlineRepository = nil
    self.watchZoneRepository = watchZoneRepository
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
    zoneSelectionKey: String = "lastSelectedZone.applications"
  ) {
    self.repository = nil
    self.offlineRepository = offlineRepository
    self.watchZoneRepository = watchZoneRepository
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
        if selectedZone == nil || !loadedZones.contains(where: { $0.id == selectedZone?.id }) {
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

  public func selectZone(_ zone: WatchZone) async {
    selectedZone = zone
    selectedStatusFilter = nil
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
