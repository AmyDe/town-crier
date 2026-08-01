import Foundation
import TownCrierDomain
import os

/// Persists and retrieves watch zones via the Town Crier API.
public final class APIWatchZoneRepository: WatchZoneRepository, Sendable {
  #if DEBUG
    private static let logger = Logger(
      subsystem: "uk.towncrierapp", category: "WatchZoneRepository")
  #endif

  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func save(_ zone: WatchZone) async throws {
    let body = CreateWatchZoneRequest(
      name: zone.name,
      latitude: zone.centre.latitude,
      longitude: zone.centre.longitude,
      radiusMetres: zone.radiusMetres,
      authorityId: zone.authorityId > 0 ? zone.authorityId : nil,
      pushEnabled: zone.pushEnabled,
      emailInstantEnabled: zone.emailInstantEnabled,
      boundary: zone.boundary.map { GeoJSONPolygon(boundary: $0) }
    )
    do {
      let _: EmptyResponse = try await apiClient.request(.post("/v1/me/watch-zones", body: body))
    } catch let domainError as DomainError {
      throw Self.normalisingCreateQuota(domainError)
    } catch {
      throw Self.normalisingCreateQuota(error.toDomainError())
    }
  }

  /// The create endpoint's only 403 is "quota exceeded", so a 403 on save means
  /// the user has hit their tier's watch-zone limit (tc-gpjk). Normalise it to
  /// `.insufficientEntitlement` — which carries the "Upgrade Required" UX — so
  /// the UI routes to the paywall rather than showing a generic "Server Error".
  /// An already-mapped `.insufficientEntitlement` (entitlement-shaped body) is
  /// passed through unchanged. The API does not return the required tier here,
  /// so we surface the next tier up.
  private static func normalisingCreateQuota(_ error: DomainError) -> DomainError {
    switch error {
    case .serverError(statusCode: 403, _):
      return .insufficientEntitlement(required: "personal")
    default:
      return error
    }
  }

  public func update(_ zone: WatchZone) async throws {
    let body = UpdateWatchZoneRequest(
      name: zone.name,
      latitude: zone.centre.latitude,
      longitude: zone.centre.longitude,
      radiusMetres: zone.radiusMetres,
      pushEnabled: zone.pushEnabled,
      emailInstantEnabled: zone.emailInstantEnabled,
      boundary: zone.boundary.map { GeoJSONPolygon(boundary: $0) }
    )
    do {
      let _: EmptyResponse = try await apiClient.request(
        .patch("/v1/me/watch-zones/\(zone.id.value)", body: body)
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
  }

  public func loadAll() async throws -> [WatchZone] {
    let result: ListWatchZonesResponse
    do {
      result = try await apiClient.request(.get("/v1/me/watch-zones"))
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
    return result.zones.compactMap { dto in
      do {
        return try dto.toDomain()
      } catch {
        #if DEBUG
          Self.logger.warning(
            "Skipping zone '\(dto.id)' ('\(dto.name)'): \(error.localizedDescription)"
          )
        #endif
        return nil
      }
    }
  }

  public func delete(_ id: WatchZoneId) async throws {
    do {
      let _: EmptyResponse = try await apiClient.request(
        .delete("/v1/me/watch-zones/\(id.value)")
      )
    } catch APIError.notFound {
      // Idempotent delete — if the zone is already gone, succeed silently
      return
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
  }
}

// MARK: - Request/Response DTOs

struct CreateWatchZoneRequest: Encodable, Sendable {
  let name: String
  let latitude: Double
  let longitude: Double
  let radiusMetres: Double
  let authorityId: Int?
  let pushEnabled: Bool
  let emailInstantEnabled: Bool
  let boundary: GeoJSONPolygon?
}

/// PATCH always sends every field, including `boundary`: unlike a partial
/// update, `update(_:)` always encodes the zone's complete desired state (see
/// `APIWatchZoneRepository.update(_:)`), matching every other field on this
/// request. A custom `encode(to:)` is required because Swift's synthesized
/// `Encodable` conformance calls `encodeIfPresent` for `Optional` properties,
/// which would omit `boundary` from the payload entirely when `nil` rather
/// than sending an explicit JSON `null` — the server's PATCH handler
/// (`decodeBoundaryUpdate`, `api-go/internal/watchzones/handler.go`) treats
/// an absent `boundary` key as "leave the shape untouched" and an explicit
/// `null` as "revert to a circle", so this distinction matters: a circle
/// zone (`boundary == nil`) always needs the latter.
struct UpdateWatchZoneRequest: Encodable, Sendable {
  let name: String
  let latitude: Double
  let longitude: Double
  let radiusMetres: Double
  let pushEnabled: Bool
  let emailInstantEnabled: Bool
  let boundary: GeoJSONPolygon?

  private enum CodingKeys: String, CodingKey {
    case name, latitude, longitude, radiusMetres, pushEnabled, emailInstantEnabled, boundary
  }

  func encode(to encoder: Encoder) throws {
    var container = encoder.container(keyedBy: CodingKeys.self)
    try container.encode(name, forKey: .name)
    try container.encode(latitude, forKey: .latitude)
    try container.encode(longitude, forKey: .longitude)
    try container.encode(radiusMetres, forKey: .radiusMetres)
    try container.encode(pushEnabled, forKey: .pushEnabled)
    try container.encode(emailInstantEnabled, forKey: .emailInstantEnabled)
    // `encode`, not `encodeIfPresent`: see the type's doc comment.
    try container.encode(boundary, forKey: .boundary)
  }
}

struct ListWatchZonesResponse: Decodable, Sendable {
  let zones: [WatchZoneSummaryDTO]
}

/// The wire shape of a custom-shape watch zone boundary: a GeoJSON Polygon
/// with a single outer ring, first vertex repeated last to close it —
/// mirrors the server's `geoJSONPolygon` (`api-go/internal/watchzones/handler.go`)
/// exactly. `coordinates` is `[ring][vertex][longitude, latitude]`, GeoJSON's
/// (lon, lat) ordering.
struct GeoJSONPolygon: Codable, Sendable {
  let type: String
  let coordinates: [[[Double]]]

  init(type: String = "Polygon", coordinates: [[[Double]]]) {
    self.type = type
    self.coordinates = coordinates
  }

  /// Builds the wire shape from a validated domain boundary.
  init(boundary: WatchZoneBoundary) {
    type = "Polygon"
    coordinates = [boundary.vertices.map { [$0.longitude, $0.latitude] }]
  }

  /// Parses the wire shape back into a validated domain boundary. Throws
  /// `DomainError.invalidWatchZoneBoundaryVertexCount` for a wrong `type`, a
  /// missing outer ring, or more than one ring (the server only ever emits a
  /// single-outer-ring Polygon; this is a defensive guard against decoding
  /// an inner/hole ring as if it were the shape), `DomainError.invalidCoordinate`
  /// for a malformed vertex pair, or whichever `DomainError`
  /// `WatchZoneBoundary.init(vertices:)` raises for an invalid ring.
  func toDomain() throws -> WatchZoneBoundary {
    guard type == "Polygon", coordinates.count == 1, let ring = coordinates.first else {
      throw DomainError.invalidWatchZoneBoundaryVertexCount
    }
    let vertices = try ring.map { vertex -> Coordinate in
      guard vertex.count == 2 else {
        throw DomainError.invalidCoordinate
      }
      return try Coordinate(latitude: vertex[1], longitude: vertex[0])
    }
    return try WatchZoneBoundary(vertices: vertices)
  }
}

struct WatchZoneSummaryDTO: Decodable, Sendable {
  let id: String
  let name: String
  let latitude: Double
  let longitude: Double
  let radiusMetres: Double
  let authorityId: Int
  let pushEnabled: Bool
  let emailInstantEnabled: Bool
  /// Server-derived: true when this zone currently exceeds the user's
  /// effective tier quota and has stopped generating notifications (GH#889
  /// P1/P2). Absent on responses predating this field, so it hydrates to
  /// `false` (an unpaused zone) — same forward/back-compat treatment as
  /// `pushEnabled`/`emailInstantEnabled` above.
  let paused: Bool
  /// The custom-shape polygon boundary, or `nil` for a circle zone
  /// (GH#1031). Absent on responses predating this field, so it hydrates to
  /// `nil` (a circle) — same back-compat treatment as `paused` above.
  let boundary: GeoJSONPolygon?

  init(
    id: String,
    name: String,
    latitude: Double,
    longitude: Double,
    radiusMetres: Double,
    authorityId: Int,
    pushEnabled: Bool = true,
    emailInstantEnabled: Bool = true,
    paused: Bool = false,
    boundary: GeoJSONPolygon? = nil
  ) {
    self.id = id
    self.name = name
    self.latitude = latitude
    self.longitude = longitude
    self.radiusMetres = radiusMetres
    self.authorityId = authorityId
    self.pushEnabled = pushEnabled
    self.emailInstantEnabled = emailInstantEnabled
    self.paused = paused
    self.boundary = boundary
  }

  init(from decoder: Decoder) throws {
    let container = try decoder.container(keyedBy: CodingKeys.self)
    self.id = try container.decode(String.self, forKey: .id)
    self.name = try container.decode(String.self, forKey: .name)
    self.latitude = try container.decode(Double.self, forKey: .latitude)
    self.longitude = try container.decode(Double.self, forKey: .longitude)
    self.radiusMetres = try container.decode(Double.self, forKey: .radiusMetres)
    self.authorityId = try container.decode(Int.self, forKey: .authorityId)
    // Per spec (T1): existing zones without these fields hydrate to true (preserves prior behaviour).
    self.pushEnabled = try container.decodeIfPresent(Bool.self, forKey: .pushEnabled) ?? true
    self.emailInstantEnabled =
      try container.decodeIfPresent(Bool.self, forKey: .emailInstantEnabled) ?? true
    // GH#889 P2: absent (older API/back-compat) hydrates to false (not paused).
    self.paused = try container.decodeIfPresent(Bool.self, forKey: .paused) ?? false
    // GH#1031: absent (older API/back-compat, or a plain circle zone) hydrates to nil.
    self.boundary = try container.decodeIfPresent(GeoJSONPolygon.self, forKey: .boundary)
  }

  private enum CodingKeys: String, CodingKey {
    case id, name, latitude, longitude, radiusMetres, authorityId
    case pushEnabled, emailInstantEnabled, paused, boundary
  }

  func toDomain() throws -> WatchZone {
    let centre = try Coordinate(latitude: latitude, longitude: longitude)
    let domainBoundary = try boundary?.toDomain()
    return try WatchZone(
      id: WatchZoneId(id),
      name: name,
      centre: centre,
      radiusMetres: radiusMetres,
      authorityId: authorityId,
      pushEnabled: pushEnabled,
      emailInstantEnabled: emailInstantEnabled,
      paused: paused,
      boundary: domainBoundary
    )
  }
}
