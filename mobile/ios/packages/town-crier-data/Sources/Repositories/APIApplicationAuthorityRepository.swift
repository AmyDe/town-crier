import Foundation
import TownCrierDomain

/// Fetches the user's application authorities via the Town Crier API.
public final class APIApplicationAuthorityRepository: ApplicationAuthorityRepository, Sendable {
  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func fetchAuthorities() async throws -> ApplicationAuthorityResult {
    do {
      let dto: ApplicationAuthoritiesDTO = try await apiClient.request(
        .get("/v1/me/application-authorities")
      )
      return dto.toDomain()
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw DomainError.networkUnavailable
    }
  }
}

// MARK: - DTOs

struct ApplicationAuthoritiesDTO: Decodable, Sendable {
  let authorities: [AuthorityItemDTO]
  let count: Int

  func toDomain() -> ApplicationAuthorityResult {
    ApplicationAuthorityResult(
      authorities: authorities.map { $0.toDomain() },
      count: count
    )
  }
}

struct AuthorityItemDTO: Decodable, Sendable {
  let id: Int
  let name: String
  let areaType: String

  func toDomain() -> LocalAuthority {
    LocalAuthority(
      code: String(id),
      name: name,
      areaType: areaType
    )
  }
}
