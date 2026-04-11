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
      throw DomainError.networkUnavailable
    }
    return dto.toDomain()
  }
}

// MARK: - Response DTOs

struct SearchResponseDTO: Decodable, Sendable {
  let applications: [SearchApplicationDTO]
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

struct SearchApplicationDTO: Decodable, Sendable {
  let name: String
  let uid: String
  let areaName: String
  let areaId: Int
  let address: String
  let postcode: String?
  let description: String
  let appType: String
  let appState: String
  let appSize: String?
  let startDate: String?
  let decidedDate: String?
  let consultedDate: String?
  let longitude: Double?
  let latitude: Double?
  let url: String?
  let link: String?
  let lastDifferent: String

  func toDomain() -> PlanningApplication {
    let status = mapAppState(appState)
    let location = mapLocation()
    let portalUrl = url.flatMap { URL(string: $0) }
    let receivedDate = parseDate(startDate) ?? Date()
    let history = synthesizeStatusHistory(status: status, receivedDate: receivedDate)

    return PlanningApplication(
      id: PlanningApplicationId(uid),
      reference: ApplicationReference(name),
      authority: LocalAuthority(code: String(areaId), name: areaName),
      status: status,
      receivedDate: receivedDate,
      description: description,
      address: address,
      location: location,
      portalUrl: portalUrl,
      statusHistory: history
    )
  }

  private func mapAppState(_ state: String) -> ApplicationStatus {
    switch state {
    case "Undecided":
      return .undecided
    case "Not Available":
      return .notAvailable
    case "Approved":
      return .approved
    case "Refused":
      return .refused
    case "Withdrawn":
      return .withdrawn
    case "Appealed":
      return .appealed
    default:
      return .unknown
    }
  }

  private func mapLocation() -> Coordinate? {
    guard let lat = latitude, let lon = longitude else { return nil }
    return try? Coordinate(latitude: lat, longitude: lon)
  }

  private func synthesizeStatusHistory(
    status: ApplicationStatus,
    receivedDate: Date
  ) -> [StatusEvent] {
    var history: [StatusEvent] = []

    if startDate != nil {
      history.append(StatusEvent(status: .undecided, date: receivedDate))
    }

    if let decidedDateString = decidedDate, let decidedDate = parseDate(decidedDateString) {
      let decidedStatus: ApplicationStatus
      switch status {
      case .approved:
        decidedStatus = .approved
      case .refused:
        decidedStatus = .refused
      case .withdrawn:
        decidedStatus = .withdrawn
      default:
        decidedStatus = status
      }
      if decidedStatus != .undecided {
        history.append(StatusEvent(status: decidedStatus, date: decidedDate))
      }
    }

    return history
  }

  private static let dateFormatter: DateFormatter = {
    let formatter = DateFormatter()
    formatter.dateFormat = "yyyy-MM-dd"
    formatter.locale = Locale(identifier: "en_US_POSIX")
    formatter.timeZone = TimeZone(identifier: "UTC")
    return formatter
  }()

  private func parseDate(_ dateString: String?) -> Date? {
    guard let dateString else { return nil }
    return Self.dateFormatter.date(from: dateString)
  }
}
