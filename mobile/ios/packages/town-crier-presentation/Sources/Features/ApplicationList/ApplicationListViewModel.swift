import Combine
import Foundation
import TownCrierDomain

/// ViewModel driving the per-zone planning-application list. Status filtering
/// is free for all subscription tiers (tc-acf0); cross-zone Saved listing now
/// lives in `SavedApplicationListViewModel`.
///
/// Owns the unread-watermark plumbing (tc-1nsa.8) when a
/// ``NotificationStateRepository`` is injected: derives the per-zone unread
/// count client-side from each row's `latestUnreadEvent` (tc-e9ox), exposes
/// the four sort modes from the spec, drives the Mark-All-Read toolbar
/// action, and supplies an Unread filter that mirrors the web bead's
/// single-select status-chip group.
@MainActor
public final class ApplicationListViewModel: ObservableObject, ErrorHandlingViewModel {
  // `internal(set)` (the default for this non-public property) rather than
  // `private(set)`: the paged load/append lives in `+Pagination` extension file
  // and mutates this. Still read-only outside the presentation module.
  @Published var applications: [PlanningApplication] = []
  @Published var selectedStatusFilter: ApplicationStatus? {
    didSet {
      // Status and Unread chips share a single-select group (spec decision #7)
      if selectedStatusFilter != nil {
        unreadOnly = false
      }
    }
  }
  @Published var unreadOnly = false {
    didSet {
      if unreadOnly {
        selectedStatusFilter = nil
      }
    }
  }
  @Published private(set) var isLoading = false
  @Published var error: DomainError?
  @Published private(set) var zones: [WatchZone] = []
  @Published private(set) var selectedZone: WatchZone?
  /// Distinct unread applications visible in the current zone. Derived from
  /// the loaded `applications` array so the chip auto-tracks zone switches
  /// and Mark-All-Read refetches without an extra round-trip. Replaces the
  /// previous `notification-state.totalUnreadCount` (global event count)
  /// wiring so the chip aligns with what each zone actually shows
  /// (GH#380 / tc-e9ox).
  public var unreadCount: Int {
    applications.filter { $0.latestUnreadEvent != nil }.count
  }
  /// Bound by the sort menu. Setter persists the choice to `UserDefaults`
  /// under `sortKey` so user intent survives relaunches (spec decision #10).
  @Published var sort: ApplicationsSort {
    didSet {
      userDefaults.set(sort.rawValue, forKey: sortKey)
    }
  }

  // `repository`/`offlineRepository`/`zone` are internal (not `private`) so the
  // `+Pagination` extension file can drive the paged fetch.
  let repository: PlanningApplicationRepository?
  let offlineRepository: OfflineAwareRepository?
  private let watchZoneRepository: WatchZoneRepository?
  private let notificationStateRepository: NotificationStateRepository?
  var zone: WatchZone?
  /// Re-entrancy guard for ``loadApplications()``. A single user action can
  /// trigger the load repeatedly — `.task` firing alongside `.refreshable`, a
  /// scenePhase change, or the view re-appearing — and each unguarded call
  /// previously spawned a duplicate fetch. SRE telemetry caught the same
  /// request firing 3-6 times within seconds, all cancelled (HTTP 499). While
  /// one load is in flight, further calls short-circuit (bd tc-eum5).
  private var isLoadInFlight = false
  /// Continuation token for the next server page, or `nil` when no more pages
  /// exist (the last page omits `X-Next-Cursor`) or the active sort is
  /// client-side. Reset to `nil` on every fresh first-page load and on any sort
  /// change so a stale cursor can never page a differently-ordered set
  /// (GH#682 slice 1). Internal so the `+Pagination` extension can drive it.
  var nextCursor: String?
  /// The sort the currently-loaded `applications` were fetched under. Lets a
  /// sort change decide whether a refetch is needed (server sorts and any
  /// transition away from one reload; a client→client switch only re-sorts in
  /// memory, as before).
  var loadedSort: ApplicationsSort?
  /// Re-entrancy guard for next-page fetches — independent of `isLoadInFlight`
  /// so an in-flight first-page load never blocks (or is blocked by) appends.
  var isPageLoadInFlight = false
  /// Server page size requested for the infinite-scroll path. Matches the API's
  /// sorted-path default (GH#682); the server clamps anything larger.
  static let pageSize = 150
  /// Trigger the next-page fetch once the appearing row is within this many rows
  /// of the end of the loaded set, so the next page lands before the user hits
  /// the bottom.
  static let prefetchThreshold = 10
  private let userDefaults: UserDefaults
  private let zoneSelectionKey: String
  private let sortKey: String

  /// Default `UserDefaults` key for the persisted sort choice. Mirrors the
  /// web localStorage key `applicationsListSort` so cross-platform users
  /// recognise the setting in support discussions.
  public static let defaultSortKey = "applicationsListSort"

  var onApplicationSelected: ((PlanningApplicationId) -> Void)?

  /// True when the zone picker should render — i.e. the user has more than one
  /// real watch zone to choose between. Single-zone users have no meaningful
  /// choice; the synthetic 'All' chip that previously kept the picker visible
  /// at one zone was retired alongside the dedicated Saved tab (tc-acf0).
  public var showZonePicker: Bool {
    zones.count > 1
  }

  /// True when the global watermark reports at least one unread notification.
  /// Drives the conditional visibility of the Unread chip and Mark-All-Read
  /// toolbar action (spec decision #8).
  public var hasUnread: Bool {
    unreadCount > 0
  }

  /// Sort modes the picker should expose right now. The `.distance` option
  /// is only meaningful relative to a chosen zone, so it's filtered out
  /// when no zone is active (multi-zone "all"-style surfaces or the
  /// transient state before the first zone is loaded). Mirrors the web
  /// sibling's picker filtering (tc-mso6 / tc-ge7j).
  public var availableSortOptions: [ApplicationsSort] {
    let active = selectedZone ?? zone
    return ApplicationsSort.allCases.filter { mode in
      mode != .distance || active != nil
    }
  }

  /// The rows to render: the loaded `applications` with the client-side
  /// status/unread filter applied. Ordering is owned entirely by the server
  /// (every sort is paged via `?sort=` since GH#682 slice 3), so there is no
  /// local re-sort — that would only ever order the pages already loaded. The
  /// status/unread filter stays client-side until slice 4.
  public var filteredApplications: [PlanningApplication] {
    filterApplications(applications)
  }

  public var isEmpty: Bool {
    filteredApplications.isEmpty && error == nil && !isLoading
  }

  public init(
    repository: PlanningApplicationRepository,
    zone: WatchZone,
    notificationStateRepository: NotificationStateRepository? = nil,
    userDefaults: UserDefaults = .standard,
    sortKey: String = defaultSortKey
  ) {
    self.repository = repository
    self.offlineRepository = nil
    self.watchZoneRepository = nil
    self.notificationStateRepository = notificationStateRepository
    self.zone = zone
    self.userDefaults = userDefaults
    self.zoneSelectionKey = ""
    self.sortKey = sortKey
    self.sort = Self.readPersistedSort(from: userDefaults, key: sortKey)
  }

  public init(
    offlineRepository: OfflineAwareRepository,
    zone: WatchZone,
    notificationStateRepository: NotificationStateRepository? = nil,
    userDefaults: UserDefaults = .standard,
    sortKey: String = defaultSortKey
  ) {
    self.repository = nil
    self.offlineRepository = offlineRepository
    self.watchZoneRepository = nil
    self.notificationStateRepository = notificationStateRepository
    self.zone = zone
    self.userDefaults = userDefaults
    self.zoneSelectionKey = ""
    self.sortKey = sortKey
    self.sort = Self.readPersistedSort(from: userDefaults, key: sortKey)
  }

  public init(
    watchZoneRepository: WatchZoneRepository,
    repository: PlanningApplicationRepository,
    notificationStateRepository: NotificationStateRepository? = nil,
    userDefaults: UserDefaults = .standard,
    zoneSelectionKey: String = "lastSelectedZone.applications",
    sortKey: String = defaultSortKey
  ) {
    self.repository = repository
    self.offlineRepository = nil
    self.watchZoneRepository = watchZoneRepository
    self.notificationStateRepository = notificationStateRepository
    self.zone = nil
    self.userDefaults = userDefaults
    self.zoneSelectionKey = zoneSelectionKey
    self.sortKey = sortKey
    self.sort = Self.readPersistedSort(from: userDefaults, key: sortKey)
  }

  public init(
    watchZoneRepository: WatchZoneRepository,
    offlineRepository: OfflineAwareRepository,
    notificationStateRepository: NotificationStateRepository? = nil,
    userDefaults: UserDefaults = .standard,
    zoneSelectionKey: String = "lastSelectedZone.applications",
    sortKey: String = defaultSortKey
  ) {
    self.repository = nil
    self.offlineRepository = offlineRepository
    self.watchZoneRepository = watchZoneRepository
    self.notificationStateRepository = notificationStateRepository
    self.zone = nil
    self.userDefaults = userDefaults
    self.zoneSelectionKey = zoneSelectionKey
    self.sortKey = sortKey
    self.sort = Self.readPersistedSort(from: userDefaults, key: sortKey)
  }

  public func loadApplications() async {
    // Drop re-entrant calls while a load is already running so a single user
    // action issues at most one fetch for the active zone (bd tc-eum5). The
    // flag is read and set synchronously before any `await`, so a second call
    // scheduled on the same actor sees it set and returns immediately.
    guard !isLoadInFlight else { return }
    isLoadInFlight = true
    defer { isLoadInFlight = false }

    isLoading = true
    error = nil
    do {
      if let watchZoneRepository {
        let loadedZones = try await watchZoneRepository.loadAll()
        zones = loadedZones
        // Refresh any in-place-edited zone (same id, new radius/centre) so its
        // updated geometry propagates downstream. If the selection has been
        // deleted — or nothing is selected yet — fall back to
        // `resolveInitialSelection`.
        if let currentId = selectedZone?.id,
          let updated = loadedZones.first(where: { $0.id == currentId }) {
          selectedZone = updated
        } else {
          resolveInitialSelection(from: loadedZones)
        }
      }
      guard let activeZone = selectedZone ?? zone else {
        applications = []
        nextCursor = nil
        isLoading = false
        return
      }
      try await loadActiveZone(activeZone)
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  public func selectZone(_ zone: WatchZone) async {
    selectedZone = zone
    selectedStatusFilter = nil
    unreadOnly = false
    userDefaults.set(zone.id.value, forKey: zoneSelectionKey)
    isLoading = true
    error = nil
    do {
      try await loadActiveZone(zone)
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  public func selectApplication(_ id: PlanningApplicationId) {
    onApplicationSelected?(id)
  }

  /// Stamps the watermark to "now" via the notification-state repository,
  /// then refetches the active zone so each row's `latestUnreadEvent` drops
  /// to `nil` (which in turn zeros `unreadCount` via the computed property).
  /// Repository failures are swallowed — a subsequent fetch will reconcile
  /// any drift. Spec decision #8 (silent optimistic).
  ///
  /// When backed by an `OfflineAwareRepository`, every cached zone is
  /// invalidated before the refetch — mark-all-read is a global mutation
  /// and a TTL-fresh per-zone cache hit would otherwise keep returning
  /// rows with the old `latestUnreadEvent`, leaving the `Unread (N)` chip
  /// stuck on the prior count (tc-e3bu).
  public func markAllRead() async {
    guard let notificationStateRepository else { return }
    do {
      try await notificationStateRepository.markAllRead()
    } catch {
      // Swallow — optimistic UI per spec decision #8.
    }
    await offlineRepository?.invalidateAllCaches()
    guard let activeZone = selectedZone ?? zone else { return }
    do {
      try await loadActiveZone(activeZone)
    } catch {
      // Refetch failure is non-fatal — the existing rows stay rendered.
    }
  }

  /// Restores the previous-session zone selection from `UserDefaults`,
  /// falling back to the first zone when nothing is persisted (or the
  /// persisted zone has been deleted).
  private func resolveInitialSelection(from zones: [WatchZone]) {
    let savedId = userDefaults.string(forKey: zoneSelectionKey)
    if let savedId,
      let savedZone = zones.first(where: { $0.id.value == savedId }) {
      selectedZone = savedZone
      return
    }
    selectedZone = zones.first
  }

  /// Internal (not `private`): the `+Pagination` extension's `loadActiveZone`
  /// uses this for the client-side sort path.
  func fetchApplications(for zone: WatchZone) async throws -> [PlanningApplication] {
    if let offlineRepository {
      return try await offlineRepository.fetchApplications(for: zone).data
    } else if let repository {
      return try await repository.fetchApplications(for: zone)
    }
    return []
  }

  private func filterApplications(
    _ applications: [PlanningApplication]
  ) -> [PlanningApplication] {
    if unreadOnly {
      return applications.filter { $0.latestUnreadEvent != nil }
    }
    if let filter = selectedStatusFilter {
      return applications.filter { $0.status == filter }
    }
    return applications
  }

  private static func readPersistedSort(
    from defaults: UserDefaults,
    key: String
  ) -> ApplicationsSort {
    if let raw = defaults.string(forKey: key),
      let parsed = ApplicationsSort(rawValue: raw) {
      return parsed
    }
    return .recentActivity
  }
}
