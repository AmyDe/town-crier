import Foundation
import TownCrierDomain

/// Searches planning applications via the Town Crier API.
///
/// Calls `GET /v1/applications/search` with query, authorityId, and page parameters.
/// Gated server-side by `Entitlement.searchApplications` (Pro tier).
public final class APISearchRepository: SearchRepository, Sendable {
  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func search(query: String, authorityId: Int, page: Int) async throws -> SearchResult {
    let dto: SearchResponseDTO
    do {
      dto = try await apiClient.request(
        .get(
          "/v1/applications/search",
          query: [
            URLQueryItem(name: "query", value: query),
            URLQueryItem(name: "authorityId", value: String(authorityId)),
            URLQueryItem(name: "page", value: String(page)),
          ]
        )
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
    return dto.toDomain()
  }
}

// MARK: - Response DTO

/// Wraps the paginated search response from `GET /v1/applications/search`.
/// Reuses ``PlanningApplicationDTO`` for individual application mapping.
struct SearchResponseDTO: Decodable, Sendable {
  let applications: [PlanningApplicationDTO]
  let total: Int
  let page: Int

  func toDomain() -> SearchResult {
    SearchResult(
      applications: applications.map { $0.toDomain() },
      total: total,
      page: page
    )
  }
}
