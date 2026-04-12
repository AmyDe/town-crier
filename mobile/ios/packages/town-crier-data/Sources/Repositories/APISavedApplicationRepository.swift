import Foundation
import TownCrierDomain

/// Manages saved (bookmarked) planning applications via the Town Crier API.
///
/// Wires:
/// - `PUT /v1/me/saved-applications/{uid}` (save, no body, 204)
/// - `DELETE /v1/me/saved-applications/{uid}` (remove, 204)
/// - `GET /v1/me/saved-applications` (load all)
///
/// The `{uid}` path parameter uses a greedy match on the API because PlanIt UIDs
/// contain slashes. The UID is passed as-is in the path.
public final class APISavedApplicationRepository: SavedApplicationRepository, Sendable {
  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func save(applicationUid: String) async throws {
    do {
      let _: EmptyResponse = try await apiClient.request(
        .put("/v1/me/saved-applications/\(applicationUid)")
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
  }

  public func remove(applicationUid: String) async throws {
    do {
      let _: EmptyResponse = try await apiClient.request(
        .delete("/v1/me/saved-applications/\(applicationUid)")
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
  }

  public func loadAll() async throws -> [SavedApplication] {
    let dto: SavedApplicationsResponseDTO
    do {
      dto = try await apiClient.request(
        .get("/v1/me/saved-applications")
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
    return dto.toDomain()
  }
}

// MARK: - Response DTOs

/// Wraps the response from `GET /v1/me/saved-applications`.
struct SavedApplicationsResponseDTO: Decodable, Sendable {
  let savedApplications: [SavedApplicationDTO]

  func toDomain() -> [SavedApplication] {
    savedApplications.map { $0.toDomain() }
  }
}

/// Individual saved application item from the API response.
struct SavedApplicationDTO: Decodable, Sendable {
  let applicationUid: String
  let savedAt: String
  let application: PlanningApplicationDTO?

  func toDomain() -> SavedApplication {
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime]
    let date = formatter.date(from: savedAt) ?? Date()
    return SavedApplication(
      applicationUid: applicationUid,
      savedAt: date,
      application: application?.toDomain()
    )
  }
}
