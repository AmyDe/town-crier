import Foundation
import TownCrierDomain

/// Fetches planning applications from the Town Crier API.
public final class APIPlanningApplicationRepository: PlanningApplicationRepository, Sendable {
  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func fetchApplications(for zone: WatchZone) async throws
    -> [PlanningApplication] {
    let dtos: [PlanningApplicationDTO]
    do {
      dtos = try await apiClient.request(
        .get("/v1/me/watch-zones/\(zone.id.value)/applications")
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
    return dtos.map { $0.toDomain() }
  }

  public func fetchApplicationsPage(
    for zone: WatchZone,
    sort: ApplicationSortOrder,
    filter: ApplicationFilter,
    cursor: String?,
    limit: Int
  ) async throws -> ApplicationPage {
    var query: [URLQueryItem] = [
      URLQueryItem(name: "sort", value: sort.rawValue),
      URLQueryItem(name: "limit", value: String(limit)),
    ]
    // Status and unread are mutually exclusive at the type level, so at most one
    // of these is ever appended — the server 400s if both arrive together. The
    // `.all` case omits both (the "All" chip / Unread off).
    switch filter {
    case .all:
      break
    case .status(let status):
      query.append(URLQueryItem(name: "status", value: status.rawValue))
    case .unread:
      query.append(URLQueryItem(name: "unread", value: "true"))
    }
    if let cursor, !cursor.isEmpty {
      query.append(URLQueryItem(name: "cursor", value: cursor))
    }

    let result: (value: [PlanningApplicationDTO], nextCursor: String?)
    do {
      result = try await apiClient.requestPaged(
        .get("/v1/me/watch-zones/\(zone.id.value)/applications", query: query)
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }

    return ApplicationPage(
      applications: result.value.map { $0.toDomain() },
      nextCursor: result.nextCursor
    )
  }

  public func fetchClusters(
    for zone: WatchZone,
    viewport: MapViewport,
    zoom: Int,
    filter: ApplicationFilter
  ) async throws -> [MapCluster] {
    var query: [URLQueryItem] = [
      URLQueryItem(name: "bbox", value: Self.bboxValue(for: viewport)),
      URLQueryItem(name: "zoom", value: String(zoom)),
    ]
    // Only a status filter is meaningful for the map clusters endpoint; `.all`
    // and `.unread` send no status param (the map has no unread filter).
    if case .status(let status) = filter {
      query.append(URLQueryItem(name: "status", value: status.rawValue))
    }

    let dtos: [MapClusterDTO]
    do {
      dtos = try await apiClient.request(
        .get("/v1/me/watch-zones/\(zone.id.value)/applications/clusters", query: query)
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
    return dtos.compactMap { $0.toDomain() }
  }

  /// Renders the viewport as the `bbox=west,south,east,north` value the server
  /// expects. Swift's `Double` interpolation is locale-independent (always uses
  /// `.` for the decimal separator), so this is safe regardless of device locale.
  private static func bboxValue(for viewport: MapViewport) -> String {
    "\(viewport.west),\(viewport.south),\(viewport.east),\(viewport.north)"
  }

  public func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication {
    let dto: PlanningApplicationDTO
    do {
      // New endpoint: /v1/applications/{authorityCode}/{**name}
      // `authorityCode` is `authority` (areaId as decimal string); `name` is the
      // PlanIt case reference. The greedy `{**name}` segment preserves slashes in
      // the reference (e.g. "22/1234/FUL"). This is a single-partition point read
      // (~1 RU) replacing the old cross-partition uid scan (tc-dzwo.1).
      dto = try await apiClient.request(
        .get("/v1/applications/\(id.authority)/\(id.name)")
      )
    } catch APIError.notFound {
      throw DomainError.applicationNotFound(id)
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
    return dto.toDomain()
  }

  public func fetchApplication(bySlug authoritySlug: String, ref: String) async throws
    -> PlanningApplication {
    let dto: PlanningApplicationDTO
    do {
      // Anonymous public read: /v1/applications/by-slug/{authoritySlug}/{**ref}.
      // The greedy `{**ref}` segment preserves slashes in the full area-prefixed
      // PlanIt name, so `ref` interpolates raw exactly like `id.name` does in the
      // by-id read above (GH #738 Slice 4).
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

// MARK: - Cluster DTO

/// Wire shape for one cell of the map clusters endpoint (GH#698). A `count > 1`
/// cell has a null `applicationId` and a multi-status `statusCounts`; a
/// `count == 1` cell carries the lone application's `{authority, name}` and a
/// single-entry `statusCounts`.
///
/// An *unsplittable* multi-member cell — members coincident or closer than the
/// finest grid cell, so zoom can never separate them — additionally carries
/// `applicationIds`, the capped list of its members' `{authority, name}` ids
/// (GH#722). It is omitted/null for every splittable cell, so `members` defaults
/// to empty and existing behaviour is unchanged.
struct MapClusterDTO: Decodable, Sendable {
  let latitude: Double
  let longitude: Double
  let count: Int
  let statusCounts: [String: Int]
  let applicationId: MemberDTO?
  let applicationIds: [MemberDTO]?

  struct MemberDTO: Decodable, Sendable {
    let authority: String
    let name: String
  }

  func toDomain() -> MapCluster? {
    guard let coordinate = try? Coordinate(latitude: latitude, longitude: longitude) else {
      return nil
    }
    // Fold unknown wire states into `.unknown`, summing any collisions so the
    // per-status counts still total `count`.
    let counts = statusCounts.reduce(into: [ApplicationStatus: Int]()) { acc, pair in
      let status = ApplicationStatus(rawValue: pair.key) ?? .unknown
      acc[status, default: 0] += pair.value
    }
    let member = applicationId.map { member in
      PlanningApplicationId(authority: member.authority, name: member.name)
    }
    let members =
      applicationIds?.map { member in
        PlanningApplicationId(authority: member.authority, name: member.name)
      } ?? []
    return MapCluster(
      coordinate: coordinate, count: count, statusCounts: counts, member: member, members: members)
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
  let latestUnreadEvent: LatestUnreadEventDTO?
  /// URL-safe authority slug for the public share URL. Present on the by-id
  /// detail read and the anonymous by-slug read; absent (`omitempty`) on the
  /// list/zone endpoints, so it decodes as optional (GH #738 Slice 4).
  let authoritySlug: String?

  func toDomain() -> PlanningApplication {
    let status = ApplicationStatus(rawValue: appState ?? "") ?? .unknown
    let location = mapLocation()
    let portalUrl = url.flatMap { URL(string: $0) }
    let receivedDate = parseDate(startDate) ?? Date()
    let history = synthesizeStatusHistory(status: status, receivedDate: receivedDate)

    return PlanningApplication(
      id: PlanningApplicationId(authority: String(areaId), name: name),
      reference: ApplicationReference(name),
      authority: LocalAuthority(code: String(areaId), name: areaName, slug: authoritySlug),
      status: status,
      receivedDate: receivedDate,
      description: description,
      address: address,
      location: location,
      portalUrl: portalUrl,
      statusHistory: history,
      latestUnreadEvent: latestUnreadEvent?.toDomain()
    )
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
      case .permitted, .conditions, .rejected, .withdrawn, .appealed:
        decidedStatus = status
      default:
        decidedStatus = .undecided
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

/// Wire shape for the per-row `latestUnreadEvent` descriptor returned by the
/// per-zone applications endpoint (tc-1nsa.3). Older API builds may omit the
/// field entirely; absence and explicit `null` both decode to `nil` on the
/// domain entity. Spec: `docs/specs/notifications-unread-watermark.md`.
struct LatestUnreadEventDTO: Decodable, Sendable {
  let type: String
  let decision: String?
  let createdAt: String

  func toDomain() -> LatestUnreadEvent? {
    // Server may emit either fractional or integer-second precision so
    // try fractional first and fall back to plain ISO-8601 — both are
    // legal ISO-8601 date-time outputs the API can produce.
    let withFractional = ISO8601DateFormatter()
    withFractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
    if let parsed = withFractional.date(from: createdAt) {
      return LatestUnreadEvent(type: type, decision: decision, createdAt: parsed)
    }
    let plain = ISO8601DateFormatter()
    plain.formatOptions = [.withInternetDateTime]
    guard let parsed = plain.date(from: createdAt) else {
      return nil
    }
    return LatestUnreadEvent(type: type, decision: decision, createdAt: parsed)
  }
}
