import Foundation
import TownCrierDomain

/// Drives the anonymous (pre-signup) map: pins near a stored coordinate, no
/// account, no watch zone, no clustering (GH#868 Phase 3). A deliberately
/// reduced feature set versus the authenticated ``MapViewModel`` — no save, no
/// status filters, no stacked-cell disambiguation — so it is a small,
/// self-contained view model rather than a shared abstraction over the two.
@MainActor
public final class AnonymousMapViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public private(set) var applications: [PlanningApplication] = []
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?
  @Published public private(set) var selectedApplication: PlanningApplication?
  @Published public private(set) var centreLat: Double
  @Published public private(set) var centreLon: Double
  @Published public private(set) var radiusMetres: Double
  /// The user-chosen live monitoring radius (GH#868 Phase 3 refinement),
  /// decoupled from `radiusMetres`: this one drives the drawn preview circle
  /// and is carried into onboarding, while `radiusMetres` drives the
  /// viewport-following near-point fetch as the user pans/zooms — mirrors
  /// the authenticated map, where the zone circle is fixed while pins follow
  /// the viewport.
  @Published public private(set) var selectedRadiusMetres: Double

  /// Mirrors the near-point endpoint's own [100, 5000]m clamp client-side
  /// (GH#868 Phase 2), so a pan/zoom gesture never requests a radius the
  /// server would silently reject anyway.
  public static let minRadiusMetres: Double = 100
  public static let maxRadiusMetres: Double = 5000
  public static let defaultLimit = 200

  /// The live radius picker's bounds. The upper bound mirrors the free
  /// tier's cap (`WatchZoneLimits(tier: .free).maxRadiusMetres`), so the
  /// radius an anonymous user picks here is exactly what a free account gets
  /// after signup — never a false preview of a bigger zone than they can
  /// actually have.
  public static let minSelectedRadiusMetres: Double = 100
  public static let maxSelectedRadiusMetres: Double = WatchZoneLimits(tier: .free).maxRadiusMetres

  /// The postcode-resolved point the radius preview circle is drawn around
  /// and the camera reframes to. Fixed for the view model's lifetime, unlike
  /// `centreLat`/`centreLon`, which follow the viewport as the user pans/
  /// zooms.
  public let anchorCoordinate: Coordinate

  private let repository: AnonymousApplicationsRepository
  private let debounceNanoseconds: UInt64
  private var pendingRegionChangeTask: Task<Void, Never>?

  /// Fired when the user takes any action deeper than the pin summary preview
  /// (full detail, save) or taps the CTA banner — the anonymous map has no
  /// such features itself, so every one of them routes to sign-up instead.
  public var onRequestSignUp: (() -> Void)?

  /// Fired whenever the live radius picker changes, so the coordinator can
  /// persist the chosen radius into `AnonymousBrowseState` ahead of the
  /// post-signup handoff into onboarding (GH#868 Phase 3 refinement).
  public var onRadiusChanged: ((Double) -> Void)?

  public init(
    repository: AnonymousApplicationsRepository,
    coordinate: Coordinate,
    radiusMetres: Double = 2000,
    debounceNanoseconds: UInt64 = 500_000_000
  ) {
    self.repository = repository
    self.centreLat = coordinate.latitude
    self.centreLon = coordinate.longitude
    self.anchorCoordinate = coordinate
    self.radiusMetres = radiusMetres
    self.selectedRadiusMetres = Self.clampSelectedRadius(radiusMetres)
    self.debounceNanoseconds = debounceNanoseconds
  }

  /// Fetches pins for the seeded coordinate/radius. Called once from the
  /// map's `.task` on first appearance.
  public func loadInitial() async {
    await fetch(latitude: centreLat, longitude: centreLon, radiusMetres: radiusMetres)
  }

  /// Called on a significant map-region change (pan/zoom settling). Clamps
  /// the reported radius to the server's bound and debounces the refetch so a
  /// continuous gesture issues a single fetch once it settles, not one per
  /// frame.
  public func regionDidChange(centreLat: Double, centreLon: Double, radiusMetres: Double) {
    let clampedRadius = min(max(radiusMetres, Self.minRadiusMetres), Self.maxRadiusMetres)
    self.centreLat = centreLat
    self.centreLon = centreLon
    self.radiusMetres = clampedRadius

    pendingRegionChangeTask?.cancel()
    let debounceDuration = debounceNanoseconds
    pendingRegionChangeTask = Task { [weak self] in
      try? await Task.sleep(nanoseconds: debounceDuration)
      guard !Task.isCancelled else { return }
      await self?.fetch(latitude: centreLat, longitude: centreLon, radiusMetres: clampedRadius)
    }
  }

  /// Test-only synchronisation: await the most recently scheduled debounced
  /// region-change refetch, mirroring
  /// `AppCoordinator.waitForPendingPostPurchasePrompt()`.
  public func waitForPendingRegionChangeRefetch() async {
    await pendingRegionChangeTask?.value
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
      // A transient refetch failure (pan/zoom) keeps the last good pins rather
      // than blanking the map; a screen-level error is surfaced only when
      // there is nothing to show yet — mirrors MapViewModel.loadClusters.
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

  /// Any deeper touch than the summary preview (full detail, save) or the CTA
  /// banner itself routes to sign-up — the anonymous map has none of those
  /// features.
  public func requestSignUp() {
    onRequestSignUp?()
  }

  /// Updates the live radius picker, clamping to
  /// `[minSelectedRadiusMetres, maxSelectedRadiusMetres]` and notifying
  /// `onRadiusChanged` so the coordinator can persist it. Called from the
  /// map's radius `Slider` (GH#868 Phase 3 refinement).
  public func updateSelectedRadius(_ metres: Double) {
    selectedRadiusMetres = Self.clampSelectedRadius(metres)
    onRadiusChanged?(selectedRadiusMetres)
  }

  private static func clampSelectedRadius(_ metres: Double) -> Double {
    min(max(metres, minSelectedRadiusMetres), maxSelectedRadiusMetres)
  }
}
