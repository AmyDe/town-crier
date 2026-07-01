import Foundation
import TownCrierDomain

/// Hits the notification read-state endpoints (see ADR 0035,
/// `docs/adr/0035-per-application-notification-read-state.md`).
///
/// `fetchState` is the single read; `markAllRead` and `markApplicationRead`
/// are fire-and-forget writes returning 204. Per-application mark-read is
/// idempotent, so callers never need to check current state first.
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

  public func markApplicationRead(applicationUid: String, authorityId: Int) async throws {
    let body = MarkApplicationsReadRequestDTO(
      applications: [
        MarkApplicationsReadRequestDTO.Item(
          applicationUid: applicationUid,
          authorityId: authorityId
        )
      ]
    )
    do {
      let _: EmptyResponse = try await apiClient.request(
        .post("/v1/me/applications/mark-read", body: body)
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
    // The backend's DotNetTime format carries fractional seconds whenever the
    // sub-second part is non-zero; parse robustly via the shared helper. The
    // epoch fallback is retained only for genuinely unparseable input.
    let date = DotNetTimeParser.date(from: lastReadAt) ?? Date(timeIntervalSince1970: 0)
    return NotificationState(
      lastReadAt: date,
      version: version,
      totalUnreadCount: totalUnreadCount
    )
  }
}

/// Wire shape of `POST /v1/me/applications/mark-read`. Modelled as an array so
/// the wire format is forward-compatible (clients send a single element today).
/// Each item is scoped by the composite `(applicationUid, authorityId)` because
/// a PlanIt reference is unique only within a council — see ADR 0035.
struct MarkApplicationsReadRequestDTO: Encodable, Sendable {
  struct Item: Encodable, Sendable {
    let applicationUid: String
    let authorityId: Int
  }

  let applications: [Item]
}
