import Foundation

/// A circular geographic area that a user monitors for planning applications.
public struct WatchZone: Equatable, Hashable, Identifiable, Sendable {
  public let id: WatchZoneId
  public let postcode: Postcode
  public let centre: Coordinate
  public let radiusMetres: Double
  public let authorityId: Int

  public init(
    id: WatchZoneId = WatchZoneId(),
    postcode: Postcode,
    centre: Coordinate,
    radiusMetres: Double,
    authorityId: Int = 0
  ) throws {
    guard radiusMetres > 0 else {
      throw DomainError.invalidWatchZoneRadius
    }
    self.id = id
    self.postcode = postcode
    self.centre = centre
    self.radiusMetres = radiusMetres
    self.authorityId = authorityId
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
