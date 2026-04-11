import Foundation
import TownCrierDomain

/// Fetches paginated notifications from the Town Crier API.
///
/// Calls `GET /v1/me/notifications` with page and pageSize query parameters.
/// Available to all tiers -- no entitlement gating.
public final class APINotificationRepository: NotificationRepository, Sendable {
  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func fetch(page: Int, pageSize: Int) async throws -> NotificationPage {
    let dto: NotificationPageDTO
    do {
      dto = try await apiClient.request(
        .get(
          "/v1/me/notifications",
          query: [
            URLQueryItem(name: "page", value: String(page)),
            URLQueryItem(name: "pageSize", value: String(pageSize)),
          ]
        )
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw DomainError.networkUnavailable
    }
    return dto.toDomain()
  }
}

// MARK: - Response DTOs

/// Wraps the paginated notification response from `GET /v1/me/notifications`.
struct NotificationPageDTO: Decodable, Sendable {
  let notifications: [NotificationItemDTO]
  let total: Int
  let page: Int

  func toDomain() -> NotificationPage {
    NotificationPage(
      notifications: notifications.map { $0.toDomain() },
      total: total,
      page: page
    )
  }
}

/// Individual notification item from the API response.
struct NotificationItemDTO: Decodable, Sendable {
  let applicationName: String
  let applicationAddress: String
  let applicationDescription: String
  let applicationType: String
  let authorityId: Int
  let createdAt: String

  func toDomain() -> NotificationItem {
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime]
    let date = formatter.date(from: createdAt) ?? Date()
    return NotificationItem(
      applicationName: applicationName,
      applicationAddress: applicationAddress,
      applicationDescription: applicationDescription,
      applicationType: applicationType,
      authorityId: authorityId,
      createdAt: date
    )
  }
}
