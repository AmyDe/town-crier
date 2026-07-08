import Foundation
import TownCrierDomain

/// Fetches a single planning application by its public share identity with no
/// account or session (GH#879 Phase 2), backed by the public
/// `GET /v1/applications/by-slug/{authoritySlug}/{ref...}` endpoint via
/// ``AnonymousURLSessionAPIClient``. Reuses ``PlanningApplicationDTO`` and its
/// `toDomain()` mapping — the wire shape is identical to the authed by-slug
/// read, `APIPlanningApplicationRepository.fetchApplication(bySlug:ref:)`,
/// which this deliberately mirrors so the two decode paths never drift.
public final class APIAnonymousApplicationDetailRepository: AnonymousApplicationDetailRepository,
  Sendable {
  private let apiClient: AnonymousURLSessionAPIClient

  public init(apiClient: AnonymousURLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func fetchApplication(bySlug authoritySlug: String, ref: String) async throws
    -> PlanningApplication {
    let dto: PlanningApplicationDTO
    do {
      // The greedy `{**ref}` segment preserves slashes in the full
      // area-prefixed PlanIt name, so `ref` interpolates raw exactly like
      // `APIPlanningApplicationRepository.fetchApplication(bySlug:ref:)`.
      dto = try await apiClient.request(
        .get("/v1/applications/by-slug/\(authoritySlug)/\(ref)")
      )
    } catch APIError.notFound {
      throw DomainError.applicationNotFound(
        PlanningApplicationId(authority: authoritySlug, name: ref)
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
    return dto.toDomain()
  }
}
