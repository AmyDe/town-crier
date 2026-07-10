import Foundation
import TownCrierDomain

/// Drives the anonymous (pre-signup) map: pins near a stored coordinate, no
/// account, no watch zone, no server-side clustering (GH#868 Phase 3). A
/// deliberately reduced feature set versus the authenticated ``MapViewModel``
/// — no save, no status filters — so it is a small, self-contained view model
/// rather than a shared abstraction over the two. It does, however, support
/// the same same-address disambiguation list (GH#877) via ``selectStack(_:)``
/// / ``selectFromStack(_:)``, reusing ``StackedApplications`` and
/// ``StackedApplicationsSheet`` from the Map feature.
///
/// GH#912 Phase 4 ("honest anon map"): previously tracked TWO decoupled
/// radii — a viewport-following `radiusMetres` driving the actual fetch as
/// the user panned, and a free-tier-capped `selectedRadiusMetres` driving
/// the drawn preview circle. That let the fetch boundary silently exceed the
/// drawn circle, so pins could render outside it. This version mirrors the
/// authenticated ``MapViewModel``'s pattern instead: ``anchorCoordinate`` and
/// ``radiusMetres`` are zone-anchored and fixed — set once at init and again
/// only by ``updateActiveZone(_:)`` — never by panning. The fetch radius IS
/// the drawn circle's radius, always exactly, so a pin can never render
/// outside the circle. Panning the map is now a pure MapKit camera gesture
/// with no ViewModel involvement at all.
@MainActor
public final class AnonymousMapViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public private(set) var applications: [PlanningApplication] = []
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?
  @Published public private(set) var selectedApplication: PlanningApplication?
  /// The applications stacked at the tapped coincident cluster (GH#877),
  /// presented as a disambiguation list via the shared
  /// ``StackedApplicationsSheet``. Nil when no stacked cluster is open.
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
  /// and pins are fetched around. Fixed except when ``updateActiveZone(_:)``
  /// changes the active device-local zone (GH#879 Phase 4) — never by
  /// panning (GH#912 Phase 4), so it is `@Published private(set)`, not `let`.
  @Published public private(set) var anchorCoordinate: Coordinate
  /// The zone's actual radius — drives both the drawn circle and the
  /// `near-point` fetch boundary, always exactly the same value, so a pin
  /// can never render outside the circle (GH#912 Phase 4). Deliberately NOT
  /// clamped to the free-tier cap: the postcode-entry screen already bounds
  /// the FIRST zone to that cap, but a zone subsequently edited larger via
  /// `DeviceLocalZoneEditorView` (bound up to `DeviceLocalZone.maxRadiusMetres`)
  /// must show its true radius, not a falsely small preview.
  @Published public private(set) var radiusMetres: Double

  public static let defaultLimit = 200

  private let repository: AnonymousApplicationsRepository
  private let debounceNanoseconds: UInt64
  /// Debounces ``updateActiveZone(_:)``'s fetch so a rapid run of zone
  /// switches collapses to a single pin swap (GH#879 Phase 4 crash fix — see
  /// that method's docs).
  private var pendingActiveZoneTask: Task<Void, Never>?

  /// Fired by the persistent CTA banner's "Create account"/"Sign in" buttons
  /// (full detail no longer requires an account — GH#879 Phase 2 replaced the
  /// summary sheet's own sign-up handoff with ``onShowApplicationDetail``).
  public var onRequestSignUp: (() -> Void)?
  /// Fired by ``presentPendingDetailIfNeeded()`` once the summary sheet has
  /// dismissed, handing the full application to the coordinator to present
  /// the detail screen (GH#879 Phase 2) — no network call, the anonymous map
  /// already holds the full `PlanningApplication` from `near-point`.
  public var onShowApplicationDetail: ((PlanningApplication) -> Void)?

  public init(
    repository: AnonymousApplicationsRepository,
    coordinate: Coordinate,
    radiusMetres: Double = 2000,
    debounceNanoseconds: UInt64 = 500_000_000
  ) {
    self.repository = repository
    self.anchorCoordinate = coordinate
    self.radiusMetres = radiusMetres
    self.debounceNanoseconds = debounceNanoseconds
  }

  /// Fetches pins for the seeded coordinate/radius. Called once from the
  /// map's `.task` on first appearance.
  public func loadInitial() async {
    await fetch(
      latitude: anchorCoordinate.latitude,
      longitude: anchorCoordinate.longitude,
      radiusMetres: radiusMetres)
  }

  private func fetch(latitude: Double, longitude: Double, radiusMetres: Double) async {
    isLoading = true
    do {
      applications = try await repository.fetchNearby(
        latitude: latitude,
        longitude: longitude,
        radiusMetres: radiusMetres,
        limit: Self.defaultLimit
      )
      error = nil
    } catch {
      // A transient refetch failure (e.g. an active-zone switch) keeps the
      // last good pins rather than blanking the map; a screen-level error is
      // surfaced only when there is nothing to show yet — mirrors
      // MapViewModel.loadClusters.
      if applications.isEmpty {
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

  /// Opens the disambiguation list for a "stacked" (same-address) cluster tap
  /// (GH#877). Unlike the authenticated map's `MapViewModel.selectStack(_:)`,
  /// this makes no repository call: `near-point` already returned every
  /// member as a full `PlanningApplication`, so the coordinator hands them
  /// straight through. `id` is derived from the member ids so re-tapping the
  /// same coincident cluster re-presents the same list rather than a spurious
  /// second sheet.
  public func selectStack(_ applications: [PlanningApplication]) {
    let id = applications.map(\.id.value).joined(separator: ",")
    stackedApplications = StackedApplications(id: id, applications: applications)
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
  /// re-init with a NEW instance is silently ignored, which is exactly what
  /// live simulator verification caught (the map stayed on the previous zone
  /// until a full relaunch). Mutating `@Published` state on the existing
  /// instance instead triggers a normal SwiftUI re-render, and preserves any
  /// live pan/zoom camera state rather than tearing the map view down.
  ///
  /// Anchor/radius update IMMEDIATELY (synchronously) so the camera and
  /// radius overlay reframe as soon as the zone switches, to the zone's
  /// ACTUAL radius (GH#912 Phase 4 — never clamped to any preview cap, so
  /// the drawn circle always matches the fetch boundary exactly). The FETCH
  /// — and therefore the full pin/annotation-set swap it produces — is
  /// debounced. This is a crash fix (GH#879 Phase 4): live simulator
  /// verification hit a MapKit-internal SIGABRT
  /// (`-[MKMapView annotationContainer:requestAddingClusterForAnnotationViews:]`
  /// → `doesNotRecognizeSelector:`) reproducibly when the active zone was
  /// switched rapidly — MapKit's own deferred clustering pass cannot
  /// tolerate the annotation set being replaced faster than it can settle.
  /// Debouncing collapses a rapid run of zone switches to a single pin swap
  /// for the LAST zone selected, rather than one overlapping swap per tap.
  ///
  /// Clears selection/stack state immediately (not debounced) — those refer
  /// to pins from the OLD zone's result set, about to be replaced, and
  /// leaving a stale summary/disambiguation sheet open over a map that has
  /// already moved to a new area would be its own bug.
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
      await self?.fetch(
        latitude: zone.centre.latitude,
        longitude: zone.centre.longitude,
        radiusMetres: zone.radiusMetres)
    }
  }

  /// Test-only synchronisation: await the most recently scheduled debounced
  /// active-zone refetch.
  public func waitForPendingActiveZoneUpdate() async {
    await pendingActiveZoneTask?.value
  }
}
