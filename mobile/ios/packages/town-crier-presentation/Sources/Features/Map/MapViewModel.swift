import Combine
import Foundation
import TownCrierDomain

/// ViewModel driving the map view with server-computed cluster aggregates
/// (GH#698). Instead of eager-draining every application in the zone, the map
/// fetches only the clusters inside the current viewport and refetches
/// (debounced) on pan/zoom, so a dense 22k zone stays smooth. Status filtering
/// is free for all subscription tiers (tc-acf0) and is applied server-side: a
/// chip change refetches clusters with `status=` rather than filtering an
/// in-memory set. `canSave` and the bookmark icon on the summary sheet remain —
/// that's the per-application save flow, not a list-level filter.
@MainActor
public final class MapViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published private(set) var clusters: [MapCluster] = []
  @Published private(set) var isLoading = false
  @Published var error: DomainError?
  @Published private(set) var selectedApplication: PlanningApplication?
  /// The application whose full detail card should open once the summary
  /// sheet has finished dismissing. Set by ``requestFullDetail()`` and
  /// consumed by ``presentPendingDetailIfNeeded()`` from the sheet's
  /// `onDismiss`, which serialises the dismiss-then-present transition and
  /// avoids SwiftUI's two-sheets-at-once race.
  @Published private(set) var pendingDetailApplication: PlanningApplication?
  /// The applications stacked at the tapped unsplittable cell, presented as a
  /// disambiguation list (GH#722). Nil when no stacked cell is open. Published
  /// only on a fully-successful read of every member (see ``selectStack(_:)``).
  @Published private(set) var stackedApplications: StackedApplications?
  /// The application whose *summary* sheet should open once the disambiguation
  /// list has finished dismissing. Set by ``selectFromStack(_:)`` and consumed by
  /// ``presentPendingSummaryIfNeeded()`` from the list sheet's `onDismiss`, which
  /// serialises the dismiss-then-present transition so the list and the summary
  /// are never on screen at once (SwiftUI's two-sheets race).
  @Published private(set) var pendingSummaryApplication: PlanningApplication?
  @Published private(set) var hasLoaded = false
  @Published private(set) var zones: [WatchZone] = []
  @Published private(set) var selectedZone: WatchZone?
  @Published private(set) var selectedStatusFilter: ApplicationStatus?
  @Published private(set) var savedApplicationUids: Set<String> = []

  @Published private(set) var centreLat: Double = 51.5074
  @Published private(set) var centreLon: Double = -0.1278
  @Published private(set) var radiusMetres: Double = 2000

  private let repository: PlanningApplicationRepository
  private let watchZoneRepository: WatchZoneRepository
  private let savedApplicationRepository: SavedApplicationRepository?
  private let userDefaults: UserDefaults
  private let zoneSelectionKey: String

  /// The viewport/zoom of the most recent cluster fetch, so a status-chip change
  /// can refetch for the same visible rect without the view re-supplying it.
  private var lastViewport: MapViewport?
  private var lastZoom: Int?

  public var canSave: Bool {
    savedApplicationRepository != nil
  }

  public var isEmpty: Bool {
    hasLoaded && clusters.isEmpty && error == nil && !isLoading
  }

  /// Whether to show the status filter chips: only once a zone has loaded, so a
  /// filter that returns zero clusters doesn't make the chips vanish (the user
  /// must be able to switch back to "All").
  public var showStatusFilters: Bool {
    hasLoaded && error == nil && selectedZone != nil
  }

  /// Whether the currently selected application is in the user's saved set.
  public var isSelectedApplicationSaved: Bool {
    guard let selected = selectedApplication else { return false }
    return savedApplicationUids.contains(selected.id.value)
  }

  public var showZonePicker: Bool {
    zones.count > 1
  }

  var onApplicationSelected: ((PlanningApplicationId) -> Void)?

  /// Fired when the user asks to open the full application detail card from the
  /// summary sheet. The coordinator wires this to its synchronous
  /// `showApplicationDetail(_ application:)` — we already hold the full
  /// `PlanningApplication`, so there is no re-fetch.
  var onShowApplicationDetail: ((PlanningApplication) -> Void)?

  public init(
    repository: PlanningApplicationRepository,
    watchZoneRepository: WatchZoneRepository,
    userDefaults: UserDefaults = .standard,
    zoneSelectionKey: String = "lastSelectedZone.map",
    savedApplicationRepository: SavedApplicationRepository? = nil
  ) {
    self.repository = repository
    self.watchZoneRepository = watchZoneRepository
    self.savedApplicationRepository = savedApplicationRepository
    self.userDefaults = userDefaults
    self.zoneSelectionKey = zoneSelectionKey
  }

  public func loadApplications() async {
    isLoading = true
    error = nil
    do {
      let loadedZones = try await watchZoneRepository.loadAll()
      zones = loadedZones
      // Always refresh `selectedZone` from the reloaded list so an in-place
      // edit (same id, new radius/centre) propagates through to the map.
      // Falling back to `resolveInitialZone` only when the id is missing
      // (zone deleted) preserves the previous-session restore behaviour.
      if let currentId = selectedZone?.id,
        let updated = loadedZones.first(where: { $0.id == currentId }) {
        selectedZone = updated
      } else {
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

      // Seed the map with the whole-zone viewport so clusters render before the
      // map view's first region-change refines them to the exact visible rect.
      let (viewport, zoom) = Self.initialViewport(
        centre: zone.centre, radiusMetres: zone.radiusMetres)
      await loadClusters(viewport: viewport, zoom: zoom)
    } catch {
      handleError(error)
    }
    isLoading = false
    hasLoaded = true
  }

  /// Fetches the cluster aggregates for a viewport at a zoom and publishes them.
  /// Called on appear (seeded from the zone) and on every debounced region
  /// change. A transient refetch failure (pan/zoom) keeps the last good clusters
  /// rather than blanking the map; a screen-level error is surfaced only when
  /// there is nothing to show yet.
  public func loadClusters(viewport: MapViewport, zoom: Int) async {
    guard let zone = selectedZone else { return }
    lastViewport = viewport
    lastZoom = zoom
    let filter: ApplicationFilter = selectedStatusFilter.map { .status($0) } ?? .all
    do {
      clusters = try await repository.fetchClusters(
        for: zone, viewport: viewport, zoom: zoom, filter: filter)
      error = nil
    } catch {
      if clusters.isEmpty {
        handleError(error)
      }
    }
  }

  /// Applies a status filter chip by refetching the current viewport's clusters
  /// server-side with `status=` (GH#698) — never by filtering a held set.
  public func applyStatusFilter(_ status: ApplicationStatus?) async {
    selectedStatusFilter = status
    guard let viewport = lastViewport, let zoom = lastZoom else { return }
    await loadClusters(viewport: viewport, zoom: zoom)
  }

  /// Loads the set of saved application UIDs so `isSelectedApplicationSaved`
  /// can be checked. No-op if no repository was provided.
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
    userDefaults.set(zone.id.value, forKey: zoneSelectionKey)
    centreLat = zone.centre.latitude
    centreLon = zone.centre.longitude
    radiusMetres = zone.radiusMetres
    isLoading = true
    error = nil
    let (viewport, zoom) = Self.initialViewport(
      centre: zone.centre, radiusMetres: zone.radiusMetres)
    await loadClusters(viewport: viewport, zoom: zoom)
    isLoading = false
  }

  private func resolveInitialZone(from zones: [WatchZone]) -> WatchZone? {
    if let savedId = userDefaults.string(forKey: zoneSelectionKey),
      let savedZone = zones.first(where: { $0.id.value == savedId }) {
      return savedZone
    }
    return zones.first
  }

  /// Routes a cluster tap. A multi-member cell does nothing here (the map view
  /// zooms into it). A single-member cell point-reads the full application by
  /// the id the cluster carried — one ~1-row read, no held set and no O(n) scan
  /// — and selects it, which presents the summary sheet.
  public func selectCluster(_ cluster: MapCluster) async {
    guard cluster.isSingleMember, let member = cluster.member else { return }
    do {
      let application = try await repository.fetchApplication(by: member)
      selectApplication(application)
    } catch {
      // A transient point-read failure leaves the map untouched; the user can
      // tap the pin again. We deliberately do not blank the map with an error.
    }
  }

  /// Routes a tap on a *stacked* (unsplittable) cell — a cluster whose members
  /// are coincident or closer than the finest grid cell, so zoom can never split
  /// them (GH#722). Point-reads every carried member concurrently (one ~1-row
  /// read each, via the same `fetchApplication(by:)` a single-pin tap uses) and
  /// publishes them as the disambiguation list, preserving the cluster's
  /// ``MapCluster/members`` order — a `TaskGroup` completes out of order, so the
  /// results are tagged with their index and reindexed.
  ///
  /// All-or-nothing on failure: if *any* member read throws, we publish nothing
  /// and leave the map untouched (no list, no error-blanking) — mirroring
  /// ``selectCluster(_:)``. A half-list would misrepresent what is at the
  /// location, and the acceptance criterion requires a fetch failure to leave map
  /// state untouched; the user can tap the bubble again. A no-op for a cell that
  /// is not stacked (the map view zooms into those instead).
  public func selectStack(_ cluster: MapCluster) async {
    guard cluster.isStacked else { return }
    let members = cluster.members
    do {
      let applications = try await withThrowingTaskGroup(
        of: (Int, PlanningApplication).self
      ) { group in
        for (index, member) in members.enumerated() {
          group.addTask { [repository] in
            (index, try await repository.fetchApplication(by: member))
          }
        }
        var collected: [(index: Int, application: PlanningApplication)] = []
        for try await pair in group {
          collected.append((index: pair.0, application: pair.1))
        }
        return collected.sorted { $0.index < $1.index }.map(\.application)
      }
      stackedApplications = StackedApplications(id: cluster.id, applications: applications)
    } catch {
      // A transient point-read failure leaves the map untouched; we deliberately
      // do not present a partial list or blank the map with an error.
    }
  }

  /// Handles a tap on a disambiguation-list row. Stashes the chosen application
  /// as ``pendingSummaryApplication`` and clears ``stackedApplications`` to
  /// dismiss the list. The list sheet's `onDismiss` then calls
  /// ``presentPendingSummaryIfNeeded()`` so the summary opens only after the list
  /// has gone — never two sheets at once (GH#722).
  public func selectFromStack(_ application: PlanningApplication) {
    pendingSummaryApplication = application
    stackedApplications = nil
  }

  /// Presents any pending stacked-row summary via ``selectApplication(_:)``,
  /// clearing the pending slot first so it fires exactly once. Invoked from the
  /// disambiguation list sheet's `onDismiss`. No-op when nothing is pending (e.g.
  /// the user swiped the list away instead of tapping a row).
  public func presentPendingSummaryIfNeeded() {
    guard let pending = pendingSummaryApplication else { return }
    pendingSummaryApplication = nil
    selectApplication(pending)
  }

  /// Dismisses the disambiguation list without selecting a row — wired to the
  /// list sheet's dismiss binding (swipe-to-dismiss).
  public func clearStack() {
    stackedApplications = nil
  }

  /// Selects an application directly, presenting the summary sheet. The map's
  /// single-pin tap reaches this via ``selectCluster(_:)`` after a point read.
  public func selectApplication(_ application: PlanningApplication) {
    selectedApplication = application
    onApplicationSelected?(application.id)
  }

  public func clearSelection() {
    selectedApplication = nil
  }

  /// Requests the full detail card for the currently selected application.
  /// Stashes the selection as `pendingDetailApplication` and clears
  /// `selectedApplication`, which dismisses the summary sheet. The MapView's
  /// sheet `onDismiss` then calls ``presentPendingDetailIfNeeded()`` so the
  /// detail card opens only after the summary has gone — never two sheets at
  /// once. No-op when nothing is selected.
  public func requestFullDetail() {
    guard let selected = selectedApplication else { return }
    pendingDetailApplication = selected
    selectedApplication = nil
  }

  /// Presents any pending detail application via ``onShowApplicationDetail``,
  /// clearing the pending slot first so it fires exactly once. Invoked from the
  /// summary sheet's `onDismiss`. No-op when nothing is pending (e.g. the user
  /// swiped the summary away instead of tapping "View full details").
  public func presentPendingDetailIfNeeded() {
    guard let pending = pendingDetailApplication else { return }
    pendingDetailApplication = nil
    onShowApplicationDetail?(pending)
  }

  /// Toggles the saved state of the currently selected application.
  /// No-op if no application is selected or no repository was provided.
  public func toggleSaveSelectedApplication() async {
    guard let repository = savedApplicationRepository,
      let selected = selectedApplication
    else { return }

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
        try await repository.save(application: selected)
        savedApplicationUids.insert(uid)
      } catch {
        // Preserve current state on failure
      }
    }
  }

  // MARK: - Viewport geometry

  /// Derives the initial whole-zone viewport and slippy zoom from a zone centre
  /// and radius. Mirrors the camera framing (a span of 2.5x the radius), so the
  /// seeded clusters cover the whole circle until the map view reports its real
  /// visible rect.
  static func initialViewport(
    centre: Coordinate, radiusMetres: Double
  ) -> (viewport: MapViewport, zoom: Int) {
    let metresPerDegreeLat = 111_320.0
    let halfSpanMetres = radiusMetres * 2.5 / 2
    let halfLatDeg = halfSpanMetres / metresPerDegreeLat
    let cosLat = max(0.01, cos(centre.latitude * .pi / 180))
    let halfLonDeg = halfSpanMetres / (metresPerDegreeLat * cosLat)
    let viewport = MapViewport(
      west: centre.longitude - halfLonDeg,
      south: centre.latitude - halfLatDeg,
      east: centre.longitude + halfLonDeg,
      north: centre.latitude + halfLatDeg)
    return (viewport, slippyZoom(forLongitudeSpanDegrees: halfLonDeg * 2))
  }

  /// Standard slippy-map zoom for a longitude span: `log2(360 / span)`, clamped
  /// to the API's accepted 0...20 range.
  static func slippyZoom(forLongitudeSpanDegrees longitudeSpan: Double) -> Int {
    guard longitudeSpan > 0 else { return 20 }
    let zoom = (log2(360.0 / longitudeSpan)).rounded()
    return max(0, min(20, Int(zoom)))
  }
}
