import Foundation
import TownCrierDomain

/// Drives the anonymous (pre-signup) map: server-computed cluster aggregates
/// across the whole radius circle, no account, no watch zone (GH#924 Phase
/// 2). Deliberately a small, self-contained view model rather than a shared
/// abstraction over the authenticated ``MapViewModel`` — but mirrors that
/// type's cluster fetch/tap-routing shape closely: ``loadClusters(viewport:zoom:)``
/// mirrors `MapViewModel.loadClusters`, ``selectCluster(_:)``/``selectStack(_:)``
/// mirror `MapViewModel.selectCluster(_:)`/`selectStack(_:)`.
///
/// GH#924 Phase 2 ("server-side anon clusters"): previously fetched the 200
/// nearest applications via `near-point` and clustered them on-device
/// (MapKit's `clusteringIdentifier`), which silently truncated dense areas —
/// a 2km circle over Kingston (1000+ applications) showed bubbles only near
/// the centre. This version instead fetches server-computed grid aggregates
/// for the visible viewport, exactly like the authenticated map, so every
/// application in the radius is represented. A cluster tap point-reads the
/// full application by its public by-slug identity via
/// ``AnonymousApplicationDetailRepository`` — cluster cells carry only
/// identity, not the full record (unlike the old `near-point` response).
///
/// GH#912 Phase 4 ("honest anon map") is preserved: ``anchorCoordinate`` and
/// ``radiusMetres`` are zone-anchored and fixed — set once at init and again
/// only by ``updateActiveZone(_:)`` — never by panning. The fetch radius IS
/// the drawn circle's radius, always exactly, so a pin can never render
/// outside the circle; only the *viewport* (the visible map rect used to
/// scope the grid aggregation) changes on pan/zoom.
@MainActor
public final class AnonymousMapViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public private(set) var clusters: [AnonymousMapCluster] = []
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?
  @Published public private(set) var selectedApplication: PlanningApplication?
  /// The applications stacked at the tapped unsplittable cluster, presented
  /// as a disambiguation list (mirrors `MapViewModel.stackedApplications`).
  /// Nil when no stacked cluster is open. Published only once every member
  /// has been successfully point-read (see ``selectStack(_:)``).
  @Published public private(set) var stackedApplications: StackedApplications?
  /// The application whose *summary* sheet should open once the
  /// disambiguation list has finished dismissing. Set by
  /// ``selectFromStack(_:)`` and consumed by
  /// ``presentPendingSummaryIfNeeded()`` from the list sheet's `onDismiss`, so
  /// the list and the summary are never on screen at once (SwiftUI's
  /// two-sheets race) — mirrors ``MapViewModel``'s handoff.
  @Published public private(set) var pendingSummaryApplication: PlanningApplication?
  /// The application whose full detail screen should present once the
  /// summary sheet has finished dismissing (GH#879 Phase 2). Set by
  /// ``requestFullDetail()`` and consumed by ``presentPendingDetailIfNeeded()``
  /// from the summary sheet's `onDismiss`, so the summary and the detail
  /// sheet are never on screen at once — mirrors ``MapViewModel``'s
  /// `pendingDetailApplication` handoff.
  @Published public private(set) var pendingDetailApplication: PlanningApplication?
  /// The point the radius circle is drawn around, the camera reframes to,
  /// and clusters are fetched around. Fixed except when
  /// ``updateActiveZone(_:)`` changes the active device-local zone — never by
  /// panning, so it is `@Published private(set)`, not `let`.
  @Published public private(set) var anchorCoordinate: Coordinate
  /// The zone's actual radius — drives both the drawn circle and the
  /// clusters fetch boundary, always exactly the same value, so a pin can
  /// never render outside the circle (GH#912 Phase 4). Deliberately NOT
  /// clamped to the free-tier cap: the postcode-entry screen already bounds
  /// the FIRST zone to that cap, but a zone subsequently edited larger via
  /// `DeviceLocalZoneEditorView` (bound up to `DeviceLocalZone.maxRadiusMetres`)
  /// must show its true radius, not a falsely small preview.
  @Published public private(set) var radiusMetres: Double

  /// The `near-point` page cap the anonymous LIST view still uses (GH#924
  /// Phase 2 moved the MAP off `near-point` entirely, onto server-side
  /// clusters — see this type's header). Kept here because
  /// `AnonymousApplicationListViewModel.defaultLimit` derives from this
  /// constant; moving it would be a needless, out-of-scope edit to that
  /// call site for a Phase 2 change that never touches the list.
  public static let defaultLimit = 200

  private let repository: AnonymousApplicationsRepository
  private let detailRepository: AnonymousApplicationDetailRepository
  private let debounceNanoseconds: UInt64
  /// Debounces ``updateActiveZone(_:)``'s fetch so a rapid run of zone
  /// switches collapses to a single cluster swap (GH#879 Phase 4 crash fix —
  /// see that method's docs).
  private var pendingActiveZoneTask: Task<Void, Never>?

  /// Fired by the persistent CTA banner's "Create account"/"Sign in" buttons
  /// (full detail no longer requires an account — GH#879 Phase 2 replaced the
  /// summary sheet's own sign-up handoff with ``onShowApplicationDetail``).
  public var onRequestSignUp: (() -> Void)?
  /// Fired by ``presentPendingDetailIfNeeded()`` once the summary sheet has
  /// dismissed, handing the full application to the coordinator to present
  /// the detail screen (GH#879 Phase 2).
  public var onShowApplicationDetail: ((PlanningApplication) -> Void)?

  public init(
    repository: AnonymousApplicationsRepository,
    detailRepository: AnonymousApplicationDetailRepository,
    coordinate: Coordinate,
    radiusMetres: Double = 2000,
    debounceNanoseconds: UInt64 = 500_000_000
  ) {
    self.repository = repository
    self.detailRepository = detailRepository
    self.anchorCoordinate = coordinate
    self.radiusMetres = radiusMetres
    self.debounceNanoseconds = debounceNanoseconds
  }

  /// Fetches clusters for the seeded coordinate/radius, deriving the initial
  /// viewport the same way ``MapViewModel/initialViewport(centre:radiusMetres:)``
  /// does (a span of 2.5x the radius, matching the camera framing) so the
  /// seeded clusters cover the whole circle until the map view's first region
  /// change refines them to the exact visible rect. Called once from the
  /// map's `.task` on first appearance.
  public func loadInitial() async {
    let (viewport, zoom) = MapViewModel.initialViewport(
      centre: anchorCoordinate, radiusMetres: radiusMetres)
    await loadClusters(viewport: viewport, zoom: zoom)
  }

  /// Fetches the cluster aggregates for a viewport at a zoom and publishes
  /// them. Called on appear (seeded from the anchor) and on every debounced
  /// region change. A transient refetch failure (pan/zoom) keeps the last
  /// good clusters rather than blanking the map; a screen-level error is
  /// surfaced only when there is nothing to show yet — mirrors
  /// `MapViewModel.loadClusters`.
  public func loadClusters(viewport: MapViewport, zoom: Int) async {
    isLoading = true
    do {
      clusters = try await repository.fetchClusters(
        latitude: anchorCoordinate.latitude,
        longitude: anchorCoordinate.longitude,
        radiusMetres: radiusMetres,
        viewport: viewport,
        zoom: zoom)
      error = nil
    } catch {
      if clusters.isEmpty {
        handleError(error)
      }
    }
    isLoading = false
  }

  public func selectApplication(_ application: PlanningApplication) {
    selectedApplication = application
  }

  public func clearSelection() {
    selectedApplication = nil
  }

  /// Routes a single-member cluster tap: point-reads the full application by
  /// its public by-slug identity via ``AnonymousApplicationDetailRepository``
  /// (the `ref` is the bare ``AnonymousClusterMember/name`` — the by-slug
  /// endpoint resolves the authority from the slug itself, so `ref` must NOT
  /// carry the authority-id prefix ``AnonymousClusterMember/value`` adds;
  /// verified live against dev: `by-slug/kingston/Kingston/26/01332/CPU` →
  /// 200, `by-slug/kingston/314/Kingston/26/01332/CPU` → 404. Same contract
  /// the authed by-slug call site uses — see
  /// `APIPlanningApplicationRepository.fetchApplication(bySlug:ref:)`'s
  /// "ref interpolates raw exactly like id.name" comment), then presents the
  /// summary sheet. No-op for a multi-member cell (the map view zooms into
  /// it or opens the disambiguation list instead — see ``selectStack(_:)``).
  /// A missing slug (should never happen — the server resolves one for every
  /// real authority) silently ignores the tap; a transient point-read
  /// failure leaves the map untouched, mirroring `MapViewModel.selectCluster(_:)`.
  public func selectCluster(_ cluster: AnonymousMapCluster) async {
    guard cluster.isSingleMember, let member = cluster.member, !member.authoritySlug.isEmpty
    else { return }
    do {
      let application = try await detailRepository.fetchApplication(
        bySlug: member.authoritySlug, ref: member.name)
      selectApplication(application)
    } catch {
      // A transient point-read failure leaves the map untouched; the user
      // can tap the pin again.
    }
  }

  /// Routes a tap on a *stacked* (unsplittable) cluster — a cell whose
  /// members are coincident or closer than the finest grid cell, so zoom can
  /// never split them. Point-reads every carried member's application
  /// concurrently by slug (one read each, via the same
  /// `fetchApplication(bySlug:ref:)` a single-pin tap uses — see
  /// ``selectCluster(_:)``'s docs for why `ref` is the bare
  /// ``AnonymousClusterMember/name``, never ``AnonymousClusterMember/value``)
  /// and publishes them as the disambiguation list, preserving the cluster's
  /// ``AnonymousMapCluster/members`` order — a `TaskGroup` completes out of
  /// order, so the results are tagged with their index and reindexed.
  /// Mirrors `MapViewModel.selectStack(_:)`.
  ///
  /// All-or-nothing: if any member has a missing slug, or any member's read
  /// throws, nothing is published and the map is left untouched (no list, no
  /// error-blanking) — the user can tap the bubble again. A no-op for a cell
  /// that is not stacked (the map view zooms into those instead).
  public func selectStack(_ cluster: AnonymousMapCluster) async {
    guard cluster.isStacked else { return }
    let members = cluster.members
    guard members.allSatisfy({ !$0.authoritySlug.isEmpty }) else { return }
    do {
      let applications = try await withThrowingTaskGroup(
        of: (Int, PlanningApplication).self
      ) { group in
        for (index, member) in members.enumerated() {
          group.addTask { [detailRepository] in
            let application = try await detailRepository.fetchApplication(
              bySlug: member.authoritySlug, ref: member.name)
            return (index, application)
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
      // A transient point-read failure leaves the map untouched; we
      // deliberately do not present a partial list or blank the map with an
      // error.
    }
  }

  /// Handles a tap on a disambiguation-list row. Stashes the chosen
  /// application as ``pendingSummaryApplication`` and clears
  /// ``stackedApplications`` to dismiss the list. The list sheet's
  /// `onDismiss` then calls ``presentPendingSummaryIfNeeded()`` so the summary
  /// opens only after the list has gone.
  public func selectFromStack(_ application: PlanningApplication) {
    pendingSummaryApplication = application
    stackedApplications = nil
  }

  /// Presents any pending stacked-row summary via ``selectApplication(_:)``,
  /// clearing the pending slot first so it fires exactly once. Invoked from
  /// the disambiguation list sheet's `onDismiss`. No-op when nothing is
  /// pending (e.g. the user swiped the list away instead of tapping a row).
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

  /// The CTA banner itself routes to sign-up — full detail no longer does
  /// (GH#879 Phase 2).
  public func requestSignUp() {
    onRequestSignUp?()
  }

  /// Requests the full detail screen for the currently selected application
  /// (GH#879 Phase 2). Stashes the selection as `pendingDetailApplication`
  /// and clears `selectedApplication`, which dismisses the summary sheet.
  /// The map view's sheet `onDismiss` then calls
  /// ``presentPendingDetailIfNeeded()`` so the detail screen opens only
  /// after the summary has gone — never two sheets at once. No-op when
  /// nothing is selected. Mirrors ``MapViewModel/requestFullDetail()``.
  public func requestFullDetail() {
    guard let selected = selectedApplication else { return }
    pendingDetailApplication = selected
    selectedApplication = nil
  }

  /// Presents any pending detail application via ``onShowApplicationDetail``,
  /// clearing the pending slot first so it fires exactly once. Invoked from
  /// the summary sheet's `onDismiss`. No-op when nothing is pending (e.g. the
  /// user swiped the summary away instead of tapping "View full details").
  public func presentPendingDetailIfNeeded() {
    guard let pending = pendingDetailApplication else { return }
    pendingDetailApplication = nil
    onShowApplicationDetail?(pending)
  }

  /// Re-centres this SAME view model on `zone` (GH#879 Phase 4 defect fix)
  /// when the active device-local zone changes — e.g. a zone-picker chip tap
  /// on the Applications tab, or a save in `DeviceLocalZoneEditorView`.
  /// Deliberately mutates in place rather than the coordinator replacing the
  /// whole `AnonymousMapViewModel` instance: `AnonymousMapView` holds this
  /// object in a `@StateObject`, which SwiftUI keeps bound to the FIRST
  /// instance passed to it — a same-position `AnonymousMapView(viewModel:)`
  /// re-init with a NEW instance is silently ignored. Mutating `@Published`
  /// state on the existing instance instead triggers a normal SwiftUI
  /// re-render, and preserves any live pan/zoom camera state rather than
  /// tearing the map view down.
  ///
  /// Anchor/radius update IMMEDIATELY (synchronously) so the camera and
  /// radius overlay reframe as soon as the zone switches, to the zone's
  /// ACTUAL radius (GH#912 Phase 4 — never clamped to any preview cap, so
  /// the drawn circle always matches the fetch boundary exactly). The FETCH
  /// — and therefore the full cluster-set swap it produces — is debounced.
  /// This is a crash fix (GH#879 Phase 4): live simulator verification hit a
  /// MapKit-internal SIGABRT reproducibly when the active zone was switched
  /// rapidly — MapKit's own deferred clustering pass cannot tolerate the
  /// annotation set being replaced faster than it can settle. Debouncing
  /// collapses a rapid run of zone switches to a single cluster swap for the
  /// LAST zone selected, rather than one overlapping swap per tap.
  ///
  /// Clears selection/stack state immediately (not debounced) — those refer
  /// to applications from the OLD zone's result set, about to be replaced,
  /// and leaving a stale summary/disambiguation sheet open over a map that
  /// has already moved to a new area would be its own bug.
  public func updateActiveZone(_ zone: DeviceLocalZone) {
    pendingActiveZoneTask?.cancel()

    selectedApplication = nil
    stackedApplications = nil
    pendingSummaryApplication = nil
    pendingDetailApplication = nil

    anchorCoordinate = zone.centre
    radiusMetres = zone.radiusMetres

    let debounceDuration = debounceNanoseconds
    pendingActiveZoneTask = Task { [weak self] in
      try? await Task.sleep(nanoseconds: debounceDuration)
      guard !Task.isCancelled else { return }
      let (viewport, zoom) = MapViewModel.initialViewport(
        centre: zone.centre, radiusMetres: zone.radiusMetres)
      await self?.loadClusters(viewport: viewport, zoom: zoom)
    }
  }

  /// Test-only synchronisation: await the most recently scheduled debounced
  /// active-zone refetch.
  public func waitForPendingActiveZoneUpdate() async {
    await pendingActiveZoneTask?.value
  }
}
