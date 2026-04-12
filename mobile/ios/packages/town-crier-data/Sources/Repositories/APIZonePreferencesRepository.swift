import Foundation
import TownCrierDomain

/// Fetches and updates per-zone notification preferences via the Town Crier API.
public final class APIZonePreferencesRepository: ZonePreferencesRepository, Sendable {
  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func fetchPreferences(zoneId: String) async throws -> ZoneNotificationPreferences {
    let dto: ZonePreferencesDTO
    do {
      dto = try await apiClient.request(.get("/v1/me/watch-zones/\(zoneId)/preferences"))
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
    return dto.toDomain()
  }

  public func updatePreferences(_ preferences: ZoneNotificationPreferences) async throws {
    let body = ZonePreferencesDTO(
      zoneId: preferences.zoneId,
      newApplications: preferences.newApplications,
      statusChanges: preferences.statusChanges,
      decisionUpdates: preferences.decisionUpdates
    )
    do {
      let _: EmptyResponse = try await apiClient.request(
        .put("/v1/me/watch-zones/\(preferences.zoneId)/preferences", body: body)
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
  }
}

// MARK: - DTO

struct ZonePreferencesDTO: Codable, Sendable {
  let zoneId: String
  let newApplications: Bool
  let statusChanges: Bool
  let decisionUpdates: Bool

  func toDomain() -> ZoneNotificationPreferences {
    ZoneNotificationPreferences(
      zoneId: zoneId,
      newApplications: newApplications,
      statusChanges: statusChanges,
      decisionUpdates: decisionUpdates
    )
  }
}
