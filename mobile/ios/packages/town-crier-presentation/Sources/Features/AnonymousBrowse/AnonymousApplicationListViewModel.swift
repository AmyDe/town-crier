import Foundation
import TownCrierDomain

/// Drives the anonymous (pre-signup) Applications tab (GH#879 Phase 3): a
/// single nearest-first page of planning applications near the stored
/// coordinate, over the same ``AnonymousApplicationsRepository`` the
/// anonymous map already uses. Deliberately reduced versus the authenticated
/// ``ApplicationListViewModel`` — no sort/filter chips, no infinite scroll,
/// no watch zone — matching the pre-resolved v1 scope decision (nearest-first
/// only; parity can follow if anonymous usage justifies it).
@MainActor
public final class AnonymousApplicationListViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public private(set) var applications: [PlanningApplication] = []
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?

  /// Mirrors ``AnonymousMapViewModel/defaultLimit`` — `near-point` returns at
  /// most this many results in nearest-first order; the anonymous list is a
  /// single bounded page, as the repository protocol is designed for.
  public static let defaultLimit = AnonymousMapViewModel.defaultLimit

  private let repository: AnonymousApplicationsRepository
  private let coordinate: Coordinate
  private let radiusMetres: Double

  /// Fired when a row is tapped, handing the already-loaded application
  /// straight to the coordinator — the established GH#879 Phase 2 handoff
  /// (``AnonymousBrowseCoordinator/onShowApplicationDetail`` ->
  /// `AppCoordinator.showAnonymousApplicationDetail`). No network call: the
  /// row's `PlanningApplication` came from this same `fetchNearby` response.
  public var onShowApplicationDetail: ((PlanningApplication) -> Void)?

  public var isEmpty: Bool {
    applications.isEmpty && error == nil && !isLoading
  }

  public init(
    repository: AnonymousApplicationsRepository,
    coordinate: Coordinate,
    radiusMetres: Double
  ) {
    self.repository = repository
    self.coordinate = coordinate
    self.radiusMetres = radiusMetres
  }

  /// Fetches (or re-fetches, for pull-to-refresh) one nearest-first page at
  /// the seeded coordinate/radius, replacing whatever was previously loaded.
  public func loadApplications() async {
    isLoading = true
    error = nil
    do {
      applications = try await repository.fetchNearby(
        latitude: coordinate.latitude,
        longitude: coordinate.longitude,
        radiusMetres: radiusMetres,
        limit: Self.defaultLimit
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
}
