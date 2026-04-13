import Foundation

/// A circular geographic area that a user monitors for planning applications.
public struct WatchZone: Equatable, Hashable, Identifiable, Sendable {
  public let id: WatchZoneId
  public let name: String
  public let centre: Coordinate
  public let radiusMetres: Double
  public let authorityId: Int

  public init(
    id: WatchZoneId = WatchZoneId(),
    name: String,
    centre: Coordinate,
    radiusMetres: Double,
    authorityId: Int = 0
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
  }

  /// Convenience initializer that derives the zone name from a validated postcode.
  public init(
    id: WatchZoneId = WatchZoneId(),
    postcode: Postcode,
    centre: Coordinate,
    radiusMetres: Double,
    authorityId: Int = 0
  ) throws {
    try self.init(
      id: id,
      name: postcode.value,
      centre: centre,
      radiusMetres: radiusMetres,
      authorityId: authorityId
    )
  }

  /// Returns true if the given coordinate falls within this watch zone.
  public func contains(_ coordinate: Coordinate) -> Bool {
    let distance = haversineDistance(from: centre, to: coordinate)
    return distance <= radiusMetres
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
