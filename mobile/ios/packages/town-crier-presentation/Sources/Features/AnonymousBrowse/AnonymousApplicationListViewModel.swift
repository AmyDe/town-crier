import Foundation
import TownCrierDomain

/// Drives the anonymous (pre-signup) Applications tab (GH#879 Phase 3): a
/// single page of planning applications, reusing the same
/// ``ApplicationListRow`` the authenticated Applications tab uses. No
/// sort/filter chips — matching the pre-resolved v1 scope decision (parity
/// can follow if anonymous usage justifies it).
///
/// GH#912 Phase 3: the fetch requests ``NearbyApplicationSortOrder/recent``
/// (most-recently-updated-first) rather than the repository's default
/// `.distance` — a fixed, silent default with no user-facing control (the
/// anonymous map keeps `.distance`; see ``AnonymousMapViewModel``, which
/// this change deliberately leaves untouched).
///
/// GH#879 Phase 4: the query is now zone-driven. The persisted active
/// ``DeviceLocalZone`` supplies the coordinate/radius; a zone picker mirrors
/// the authed ``ApplicationListViewModel``'s pattern
/// (`showZonePicker`/`zones`/`selectedZone`/`selectZone(_:)`), but over
/// device-local zones instead of server-side watch zones.
/// `fallbackCoordinate`/`fallbackRadiusMetres` back the query only in the
/// practically-unreachable case no device-local zone exists at all (e.g.
/// before the legacy-state migration has ever run).
///
/// GH#888: the on-device cap dropped to one zone, so the picker row is now
/// always shown once any zone exists (``showZonePicker``), rather than only
/// once a second zone made a picker a meaningful choice. A trailing
/// "Add area" chip renders alongside it, routing to the same sign-up CTA
/// sheet every other anonymous zone-limit affordance uses
/// (``isSignUpCTAPresented``) — adding a second area now requires an
/// account.
@MainActor
public final class AnonymousApplicationListViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public private(set) var applications: [PlanningApplication] = []
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?
  @Published public private(set) var zones: [DeviceLocalZone] = []
  @Published public private(set) var selectedZone: DeviceLocalZone?
  @Published public var isSignUpCTAPresented = false

  /// Mirrors ``AnonymousMapViewModel/defaultLimit`` — `near-point` returns at
  /// most this many results in nearest-first order; the anonymous list is a
  /// single bounded page, as the repository protocol is designed for.
  public static let defaultLimit = AnonymousMapViewModel.defaultLimit

  private let repository: AnonymousApplicationsRepository
  private let zoneRepository: DeviceLocalZoneRepository
  private let fallbackCoordinate: Coordinate
  private let fallbackRadiusMetres: Double

  /// Fired when a row is tapped, handing the already-loaded application
  /// straight to the coordinator — the established GH#879 Phase 2 handoff
  /// (``AnonymousBrowseCoordinator/onShowApplicationDetail`` ->
  /// `AppCoordinator.showAnonymousApplicationDetail`). No network call: the
  /// row's `PlanningApplication` came from this same `fetchNearby` response.
  public var onShowApplicationDetail: ((PlanningApplication) -> Void)?

  /// Fired whenever the zone picker's selection changes (GH#879 Phase 4), so
  /// the coordinator can re-centre the Map tab to match.
  public var onActiveZoneChanged: ((DeviceLocalZone) -> Void)?

  /// Fired when the user confirms the "Add area" chip's sign-up CTA
  /// (GH#888). Wired by the coordinator to the same single sign-up/sign-in
  /// entry point every anonymous CTA uses.
  public var onRequestSignUp: (() -> Void)?

  public var isEmpty: Bool {
    applications.isEmpty && error == nil && !isLoading
  }

  /// True when the zone chip row should render (GH#888): unlike the authed
  /// `ApplicationListViewModel.showZonePicker`, a single zone IS shown —
  /// it's paired with a trailing "Add area" chip, so even one zone is a
  /// meaningful row (the chip and the CTA, not a choice between zones).
  public var showZonePicker: Bool {
    !zones.isEmpty
  }

  public init(
    repository: AnonymousApplicationsRepository,
    zoneRepository: DeviceLocalZoneRepository,
    fallbackCoordinate: Coordinate,
    fallbackRadiusMetres: Double
  ) {
    self.repository = repository
    self.zoneRepository = zoneRepository
    self.fallbackCoordinate = fallbackCoordinate
    self.fallbackRadiusMetres = fallbackRadiusMetres
  }

  /// Fetches (or re-fetches, for pull-to-refresh) one nearest-first page at
  /// the active zone's coordinate/radius, replacing whatever was previously
  /// loaded. Re-reads the zone list from the repository on every call so
  /// edits made on the Zones tab are picked up without a separate refresh
  /// signal.
  public func loadApplications() async {
    isLoading = true
    error = nil
    refreshZones()
    let (latitude, longitude, radiusMetres) = activeQueryLocation()
    do {
      applications = try await repository.fetchNearby(
        latitude: latitude,
        longitude: longitude,
        radiusMetres: radiusMetres,
        limit: Self.defaultLimit,
        sort: .recent
      )
    } catch {
      applications = []
      handleError(error)
    }
    isLoading = false
  }

  /// Switches the active zone (a picker chip tap): persists the choice,
  /// notifies ``onActiveZoneChanged`` so the Map tab re-centres, and
  /// re-fetches the list at the new zone's coordinate/radius.
  public func selectZone(_ zone: DeviceLocalZone) async {
    selectedZone = zone
    zoneRepository.setActiveZoneId(zone.id)
    onActiveZoneChanged?(zone)

    isLoading = true
    error = nil
    do {
      applications = try await repository.fetchNearby(
        latitude: zone.centre.latitude,
        longitude: zone.centre.longitude,
        radiusMetres: zone.radiusMetres,
        limit: Self.defaultLimit,
        sort: .recent
      )
    } catch {
      applications = []
      handleError(error)
    }
    isLoading = false
  }

  public func selectApplication(_ application: PlanningApplication) {
    onShowApplicationDetail?(application)
  }

  /// The trailing "Add area" chip (GH#888) — the on-device cap is one zone,
  /// so adding another always routes to the sign-up CTA rather than opening
  /// a device-local editor.
  public func requestAddArea() {
    isSignUpCTAPresented = true
  }

  public func dismissSignUpCTA() {
    isSignUpCTAPresented = false
  }

  public func confirmSignUp() {
    isSignUpCTAPresented = false
    onRequestSignUp?()
  }

  private func refreshZones() {
    zones = zoneRepository.loadAll()
    if let activeId = zoneRepository.activeZoneId() {
      selectedZone = zones.first { $0.id == activeId } ?? zones.first
    } else {
      selectedZone = zones.first
    }
  }

  private func activeQueryLocation() -> (
    latitude: Double, longitude: Double, radiusMetres: Double
  ) {
    guard let zone = selectedZone else {
      return (fallbackCoordinate.latitude, fallbackCoordinate.longitude, fallbackRadiusMetres)
    }
    return (zone.centre.latitude, zone.centre.longitude, zone.radiusMetres)
  }
}
