import Foundation
import TownCrierDomain

/// Fetches planning applications near a point with no account or session
/// (GH#868 Phase 3), backed by the public `GET /v1/applications/near-point`
/// endpoint via ``AnonymousURLSessionAPIClient``.
///
/// `NearbyResult` (the endpoint's wire shape) is field-for-field identical to
/// ``PlanningApplicationDTO`` minus `latestUnreadEvent`/`authoritySlug` — both
/// already optional there — so this deliberately reuses that DTO and its
/// existing `toDomain()` mapping (date parsing, status mapping, history
/// synthesis) rather than duplicating it.
public final class APIAnonymousApplicationsRepository: AnonymousApplicationsRepository, Sendable {
  private let apiClient: AnonymousURLSessionAPIClient

  public init(apiClient: AnonymousURLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func fetchNearby(
    latitude: Double,
    longitude: Double,
    radiusMetres: Double,
    limit: Int
  ) async throws -> [PlanningApplication] {
    let dtos: [PlanningApplicationDTO]
    do {
      dtos = try await apiClient.request(
        .get(
          "/v1/applications/near-point",
          query: [
            // Swift's Double interpolation/String(_:) is locale-independent
            // (always uses `.` for the decimal separator), mirroring
            // APIPlanningApplicationRepository.bboxValue's precedent.
            URLQueryItem(name: "lat", value: String(latitude)),
            URLQueryItem(name: "lng", value: String(longitude)),
            URLQueryItem(name: "radius", value: String(radiusMetres)),
            URLQueryItem(name: "limit", value: String(limit)),
          ])
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
    return dtos.map { $0.toDomain() }
  }
}
