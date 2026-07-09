import Foundation

/// A circular geographic area that a user monitors for planning applications.
public struct WatchZone: Equatable, Hashable, Identifiable, Sendable {
  public let id: WatchZoneId
  public let name: String
  public let centre: Coordinate
  public let radiusMetres: Double
  public let authorityId: Int
  public let pushEnabled: Bool
  public let emailInstantEnabled: Bool
  /// Whether this zone currently exceeds the user's effective tier quota and
  /// has stopped generating new notifications (GH#889 P1/P2). Purely a
  /// server-derived display flag — never computed or mutated by the client.
  /// A paused zone remains fully listed, editable, and deletable; it is
  /// automatically revived (server-side, with no client action) once the
  /// user upgrades or deletes older zones.
  public let paused: Bool

  public init(
    id: WatchZoneId = WatchZoneId(),
    name: String,
    centre: Coordinate,
    radiusMetres: Double,
    authorityId: Int = 0,
    pushEnabled: Bool = true,
    emailInstantEnabled: Bool = true,
    paused: Bool = false
  ) throws {
    let trimmed = name.trimmingCharacters(in: .whitespaces)
    guard !trimmed.isEmpty else {
      throw DomainError.invalidWatchZoneName
    }
    guard radiusMetres > 0 else {
      throw DomainError.invalidWatchZoneRadius
    }
    self.id = id
    self.name = trimmed
    self.centre = centre
    self.radiusMetres = radiusMetres
    self.authorityId = authorityId
    self.pushEnabled = pushEnabled
    self.emailInstantEnabled = emailInstantEnabled
    self.paused = paused
  }

  /// Convenience initializer that derives the zone name from a validated postcode.
  public init(
    id: WatchZoneId = WatchZoneId(),
    postcode: Postcode,
    centre: Coordinate,
    radiusMetres: Double,
    authorityId: Int = 0,
    pushEnabled: Bool = true,
    emailInstantEnabled: Bool = true,
    paused: Bool = false
  ) throws {
    try self.init(
      id: id,
      name: postcode.value,
      centre: centre,
      radiusMetres: radiusMetres,
      authorityId: authorityId,
      pushEnabled: pushEnabled,
      emailInstantEnabled: emailInstantEnabled,
      paused: paused
    )
  }

  /// Returns true if the given coordinate falls within this watch zone.
  public func contains(_ coordinate: Coordinate) -> Bool {
    let distance = haversineDistance(from: centre, to: coordinate)
    return distance <= radiusMetres
  }

  /// Great-circle distance in metres from this zone's centre to the given
  /// coordinate. Used by the Applications screen's distance sort
  /// (tc-mso6) and any future "near me"-style features that need a
  /// stable comparator across the domain layer.
  public func distance(to coordinate: Coordinate) -> Double {
    haversineDistance(from: centre, to: coordinate)
  }

  private func haversineDistance(from a: Coordinate, to b: Coordinate) -> Double {
    let earthRadius: Double = 6_371_000
    let dLat = (b.latitude - a.latitude) * .pi / 180
    let dLon = (b.longitude - a.longitude) * .pi / 180
    let lat1 = a.latitude * .pi / 180
    let lat2 = b.latitude * .pi / 180

    let sinHalfDLat = sin(dLat / 2)
    let sinHalfDLon = sin(dLon / 2)
    let h = sinHalfDLat * sinHalfDLat + cos(lat1) * cos(lat2) * sinHalfDLon * sinHalfDLon
    return 2 * earthRadius * asin(sqrt(h))
  }
}
