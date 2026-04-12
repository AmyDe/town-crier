import Foundation
import TownCrierDomain

/// Manages the user's server-side profile via the Town Crier API.
public final class APIUserProfileRepository: UserProfileRepository, Sendable {
  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func create() async throws -> ServerProfile {
    do {
      let dto: ServerProfileDTO = try await apiClient.request(.post("/v1/me"))
      return dto.toDomain()
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
  }

  public func fetch() async throws -> ServerProfile? {
    do {
      let dto: ServerProfileDTO = try await apiClient.request(.get("/v1/me"))
      return dto.toDomain()
    } catch APIError.notFound {
      return nil
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
  }

  public func update(
    pushEnabled: Bool,
    digestDay: DayOfWeek,
    emailDigestEnabled: Bool
  ) async throws -> ServerProfile {
    let body = UpdateProfileRequest(
      pushEnabled: pushEnabled,
      digestDay: digestDay,
      emailDigestEnabled: emailDigestEnabled
    )
    do {
      let dto: ServerProfileDTO = try await apiClient.request(
        .patch("/v1/me", body: body)
      )
      return dto.toDomain()
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
  }

  public func delete() async throws {
    do {
      let _: EmptyResponse = try await apiClient.request(.delete("/v1/me"))
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
  }
}

// MARK: - DTOs

struct ServerProfileDTO: Decodable, Sendable {
  let userId: String
  let tier: String
  let pushEnabled: Bool
  let digestDay: String
  let emailDigestEnabled: Bool

  enum CodingKeys: String, CodingKey {
    case userId
    case tier
    case pushEnabled
    case digestDay
    case emailDigestEnabled
  }

  init(from decoder: any Decoder) throws {
    let container = try decoder.container(keyedBy: CodingKeys.self)
    userId = try container.decode(String.self, forKey: .userId)
    tier = try container.decode(String.self, forKey: .tier)
    pushEnabled = try container.decode(Bool.self, forKey: .pushEnabled)
    digestDay = try container.decodeIfPresent(String.self, forKey: .digestDay) ?? "Monday"
    emailDigestEnabled =
      try container.decodeIfPresent(Bool.self, forKey: .emailDigestEnabled)
      ?? true
  }

  func toDomain() -> ServerProfile {
    ServerProfile(
      userId: userId,
      tier: SubscriptionTier(rawValue: tier.lowercased()) ?? .free,
      pushEnabled: pushEnabled,
      digestDay: DayOfWeek(rawValue: digestDay) ?? .monday,
      emailDigestEnabled: emailDigestEnabled
    )
  }
}

struct UpdateProfileRequest: Encodable, Sendable {
  let pushEnabled: Bool
  let digestDay: DayOfWeek
  let emailDigestEnabled: Bool

  enum CodingKeys: String, CodingKey {
    case pushEnabled
    case digestDay
    case emailDigestEnabled
  }
}
