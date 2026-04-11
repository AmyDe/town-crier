import Foundation
import TownCrierDomain

/// Fetches planning applications from the Town Crier API.
public final class APIPlanningApplicationRepository: PlanningApplicationRepository, Sendable {
  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func fetchApplications(for authority: LocalAuthority) async throws -> [PlanningApplication]
  {
    let dtos: [PlanningApplicationDTO]
    do {
      dtos = try await apiClient.request(
        .get("/v1/applications", query: [URLQueryItem(name: "authorityId", value: authority.code)])
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw DomainError.networkUnavailable
    }
    return dtos.map { $0.toDomain() }
  }

  public func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication {
    let dto: PlanningApplicationDTO
    do {
      dto = try await apiClient.request(
        .get("/v1/applications/\(id.value)")
      )
    } catch APIError.notFound {
      throw DomainError.applicationNotFound(id)
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw DomainError.networkUnavailable
    }
    return dto.toDomain()
  }
}

// MARK: - Response DTO

struct PlanningApplicationDTO: Decodable, Sendable {
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
    case "Under Review":
      return .underReview
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
      history.append(StatusEvent(status: .underReview, date: receivedDate))
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
      if decidedStatus != .underReview {
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
