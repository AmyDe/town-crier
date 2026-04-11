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
}
