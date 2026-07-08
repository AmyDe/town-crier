/// Port for the anonymous (pre-signup) map's data access (GH#868 Phase 3):
/// fetches planning applications near a point with no account or session,
/// backed by the public `GET /v1/applications/near-point` endpoint.
public protocol AnonymousApplicationsRepository: Sendable {
  /// Fetches one nearest-first page of planning applications within
  /// `radiusMetres` of (`latitude`, `longitude`), capped at `limit`. No
  /// clustering or infinite scroll — the anonymous map is a deliberately
  /// reduced feature set, so a single bounded page per query is sufficient.
  func fetchNearby(
    latitude: Double,
    longitude: Double,
    radiusMetres: Double,
    limit: Int
  ) async throws -> [PlanningApplication]
}
