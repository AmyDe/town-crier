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

  public func save(application: PlanningApplication) async throws {
    let body = SaveApplicationRequestDTO(application: application)
    do {
      let _: EmptyResponse = try await apiClient.request(
        .put("/v1/me/saved-applications/\(application.id.value)", body: body)
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
    let dtos: [SavedApplicationDTO]
    do {
      dtos = try await apiClient.request(
        .get("/v1/me/saved-applications")
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
    return dtos.map { $0.toDomain() }
  }
}

// MARK: - Request DTO

/// Body of `PUT /v1/me/saved-applications/{uid}`. Carries the full
/// PlanningApplication payload so the API can upsert the canonical record into
/// Cosmos at save time (see bead tc-if12). Field names mirror the API's
/// `SaveApplicationRequest` record exactly so the camel-case JSON encoding
/// produced by the default `JSONEncoder` matches what the API deserializes.
struct SaveApplicationRequestDTO: Encodable, Sendable {
  let name: String
  let uid: String
  let areaName: String
  let areaId: Int
  let address: String
  let postcode: String?
  let description: String
  let appType: String?
  let appState: String?
  let appSize: String?
  let startDate: String?
  let decidedDate: String?
  let consultedDate: String?
  let longitude: Double?
  let latitude: Double?
  let url: String?
  let link: String?
  let lastDifferent: String

  init(application: PlanningApplication) {
    self.name = application.reference.value
    self.uid = application.id.value
    self.areaName = application.authority.name
    self.areaId = Int(application.authority.code) ?? 0
    self.address = application.address
    self.postcode = nil
    self.description = application.description
    self.appType = nil
    self.appState = application.status == .unknown ? nil : application.status.rawValue
    self.appSize = nil
    self.startDate = Self.dateFormatter.string(from: application.receivedDate)
    self.decidedDate = application.statusHistory
      .first { $0.status != .undecided }
      .map { Self.dateFormatter.string(from: $0.date) }
    self.consultedDate = nil
    self.longitude = application.location?.longitude
    self.latitude = application.location?.latitude
    self.url = application.portalUrl?.absoluteString
    self.link = nil
    // PlanIt's `lastDifferent` bookkeeping is not retained on iOS; the API
    // accepts the value as the canonical "this is what I observed" timestamp.
    // Using `now` is safe — it is overwritten on the next poll cycle that sees
    // a fresher PlanIt entry.
    self.lastDifferent = Self.iso8601Formatter.string(from: Date())
  }

  private static let dateFormatter: DateFormatter = {
    let formatter = DateFormatter()
    formatter.dateFormat = "yyyy-MM-dd"
    formatter.locale = Locale(identifier: "en_US_POSIX")
    formatter.timeZone = TimeZone(identifier: "UTC")
    return formatter
  }()

  private static let iso8601Formatter: DateFormatter = {
    let formatter = DateFormatter()
    formatter.dateFormat = "yyyy-MM-dd'T'HH:mm:ss.SSSXXXXX"
    formatter.locale = Locale(identifier: "en_US_POSIX")
    formatter.timeZone = TimeZone(identifier: "UTC")
    return formatter
  }()
}

// MARK: - Response DTOs

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
