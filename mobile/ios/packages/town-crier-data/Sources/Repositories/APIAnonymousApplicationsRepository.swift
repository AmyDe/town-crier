import Foundation
import TownCrierDomain

/// Fetches planning applications near a point with no account or session
/// (GH#868 Phase 3), backed by the public `GET /v1/applications/near-point`
/// endpoint via ``AnonymousURLSessionAPIClient``.
///
/// `NearbyResult` (the endpoint's wire shape) is field-for-field identical to
/// ``PlanningApplicationDTO`` minus `latestUnreadEvent`/`authoritySlug` — both
/// already optional there — so this deliberately reuses that DTO and its
/// existing `toDomain()` mapping (date parsing, status mapping, history
/// synthesis) rather than duplicating it.
public final class APIAnonymousApplicationsRepository: AnonymousApplicationsRepository, Sendable {
  private let apiClient: AnonymousURLSessionAPIClient

  public init(apiClient: AnonymousURLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func fetchNearby(
    latitude: Double,
    longitude: Double,
    radiusMetres: Double,
    limit: Int,
    sort: NearbyApplicationSortOrder
  ) async throws -> [PlanningApplication] {
    let dtos: [PlanningApplicationDTO]
    do {
      dtos = try await apiClient.request(
        .get(
          "/v1/applications/near-point",
          query: [
            // Swift's Double interpolation/String(_:) is locale-independent
            // (always uses `.` for the decimal separator), mirroring
            // APIPlanningApplicationRepository.bboxValue's precedent.
            URLQueryItem(name: "lat", value: String(latitude)),
            URLQueryItem(name: "lng", value: String(longitude)),
            URLQueryItem(name: "radius", value: String(radiusMetres)),
            URLQueryItem(name: "limit", value: String(limit)),
            URLQueryItem(name: "sort", value: sort.rawValue),
          ])
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
    return dtos.map { $0.toDomain() }
  }

  public func fetchClusters(
    latitude: Double,
    longitude: Double,
    radiusMetres: Double,
    viewport: MapViewport,
    zoom: Int
  ) async throws -> [AnonymousMapCluster] {
    let dtos: [AnonymousMapClusterDTO]
    do {
      dtos = try await apiClient.request(
        .get(
          "/v1/applications/clusters",
          query: [
            URLQueryItem(name: "lat", value: String(latitude)),
            URLQueryItem(name: "lng", value: String(longitude)),
            URLQueryItem(name: "radius", value: String(radiusMetres)),
            URLQueryItem(name: "bbox", value: Self.bboxValue(for: viewport)),
            URLQueryItem(name: "zoom", value: String(zoom)),
          ])
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
    return dtos.compactMap { $0.toDomain() }
  }

  /// Renders the viewport as the `bbox=west,south,east,north` value the
  /// server expects. Swift's `Double` interpolation is locale-independent
  /// (always uses `.` for the decimal separator), so this is safe regardless
  /// of device locale — mirrors `APIPlanningApplicationRepository.bboxValue`.
  private static func bboxValue(for viewport: MapViewport) -> String {
    "\(viewport.west),\(viewport.south),\(viewport.east),\(viewport.north)"
  }
}

// MARK: - Cluster DTO

/// Wire shape for one cell of the anonymous map clusters endpoint (GH#924
/// Phase 2) — field-for-field identical to `MapClusterDTO` plus
/// `authoritySlug` on each carried member (the anonymous by-slug point-read
/// needs it; the authed map instead point-reads by id). A `count > 1` cell
/// has a null `applicationId` and a multi-status `statusCounts`; a
/// `count == 1` cell carries the lone application's identity and a
/// single-entry `statusCounts`. An *unsplittable* multi-member cell
/// additionally carries `applicationIds`, the capped list of its members'
/// identities — omitted/null for every splittable cell, so `members`
/// defaults to empty.
struct AnonymousMapClusterDTO: Decodable, Sendable {
  let latitude: Double
  let longitude: Double
  let count: Int
  let statusCounts: [String: Int]
  let applicationId: MemberDTO?
  let applicationIds: [MemberDTO]?

  struct MemberDTO: Decodable, Sendable {
    let authority: String
    let name: String
    /// Present on every real authority (the static authorities table covers
    /// them all); decodes to an empty string on the practically-unreachable
    /// resolver-miss case, mirroring the server's own logged fallback.
    let authoritySlug: String?
  }

  func toDomain() -> AnonymousMapCluster? {
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
      AnonymousClusterMember(
        authority: member.authority,
        name: member.name,
        authoritySlug: member.authoritySlug ?? "")
    }
    let members =
      applicationIds?.map { member in
        AnonymousClusterMember(
          authority: member.authority,
          name: member.name,
          authoritySlug: member.authoritySlug ?? "")
      } ?? []
    return AnonymousMapCluster(
      coordinate: coordinate, count: count, statusCounts: counts, member: member, members: members)
  }
}
