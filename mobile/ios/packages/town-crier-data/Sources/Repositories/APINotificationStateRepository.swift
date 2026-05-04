import Foundation
import TownCrierDomain

/// Hits the three watermark endpoints described in spec
/// `docs/specs/notifications-unread-watermark.md#api-surface`.
///
/// `fetchState` is the single read; `markAllRead` and `advance` are
/// fire-and-forget writes returning 204. The server enforces watermark
/// monotonicity so callers never need to compare timestamps locally.
public final class APINotificationStateRepository: NotificationStateRepository, Sendable {
  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func fetchState() async throws -> NotificationState {
    let dto: NotificationStateDTO
    do {
      dto = try await apiClient.request(.get("/v1/me/notification-state"))
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
    return dto.toDomain()
  }

  public func markAllRead() async throws {
    do {
      let _: EmptyResponse = try await apiClient.request(
        .post("/v1/me/notification-state/mark-all-read")
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
  }

  public func advance(asOf: Date) async throws {
    let body = AdvanceNotificationStateRequestDTO(asOf: asOf)
    do {
      let _: EmptyResponse = try await apiClient.request(
        .post("/v1/me/notification-state/advance", body: body)
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
  }
}

// MARK: - DTOs

/// Wire shape of `GET /v1/me/notification-state`. The server emits ISO-8601 for
/// `lastReadAt`; we decode through a string and parse explicitly so the
/// behaviour is independent of `JSONDecoder.dateDecodingStrategy` (which
/// defaults to `.deferredToDate` and would otherwise reject the format).
struct NotificationStateDTO: Decodable, Sendable {
  let lastReadAt: String
  let version: Int
  let totalUnreadCount: Int

  func toDomain() -> NotificationState {
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime]
    let date = formatter.date(from: lastReadAt) ?? Date(timeIntervalSince1970: 0)
    return NotificationState(
      lastReadAt: date,
      version: version,
      totalUnreadCount: totalUnreadCount
    )
  }
}

/// Wire shape of `POST /v1/me/notification-state/advance`. The server reads
/// `asOf` as ISO-8601, so we encode the `Date` through a formatter rather
/// than relying on `JSONEncoder.dateEncodingStrategy` (which defaults to
/// `.deferredToDate` — a numeric reference-date interval, not ISO-8601).
struct AdvanceNotificationStateRequestDTO: Encodable, Sendable {
  let asOf: String

  init(asOf: Date) {
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime]
    self.asOf = formatter.string(from: asOf)
  }
}
