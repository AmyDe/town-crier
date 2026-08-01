import Foundation

/// A geographic coordinate (latitude/longitude pair).
public struct Coordinate: Equatable, Hashable, Sendable {
  public let latitude: Double
  public let longitude: Double

  public init(latitude: Double, longitude: Double) throws {
    guard (-90...90).contains(latitude), (-180...180).contains(longitude) else {
      throw DomainError.invalidCoordinate
    }
    self.latitude = latitude
    self.longitude = longitude
  }

  /// Great-circle distance to another coordinate, in metres. Shared by
  /// `WatchZone.distance(to:)`/`contains(_:)` and
  /// `WatchZoneBoundary.enclosingRadiusMetres`, which both need the same
  /// haversine calculation.
  public func distanceMetres(to other: Coordinate) -> Double {
    Self.haversineMetres(
      latitude1: latitude, longitude1: longitude,
      latitude2: other.latitude, longitude2: other.longitude
    )
  }

  /// Great-circle distance between two raw lat/lon pairs, in metres. Takes
  /// raw values (rather than two `Coordinate`s) so callers with an
  /// unvalidated pair — e.g. `WatchZoneBoundary.centroid`, which is a
  /// derived point rather than a user-supplied one — don't need to
  /// re-validate it through `Coordinate.init` just to measure a distance.
  static func haversineMetres(
    latitude1: Double, longitude1: Double,
    latitude2: Double, longitude2: Double
  ) -> Double {
    let earthRadiusMetres = 6_371_000.0
    let degToRad = Double.pi / 180
    let phi1 = latitude1 * degToRad
    let phi2 = latitude2 * degToRad
    let deltaPhi = (latitude2 - latitude1) * degToRad
    let deltaLambda = (longitude2 - longitude1) * degToRad
    let haversine =
      sin(deltaPhi / 2) * sin(deltaPhi / 2)
      + cos(phi1) * cos(phi2) * sin(deltaLambda / 2) * sin(deltaLambda / 2)
    let angularDistance = 2 * atan2(sqrt(haversine), sqrt(1 - haversine))
    return earthRadiusMetres * angularDistance
  }
}
