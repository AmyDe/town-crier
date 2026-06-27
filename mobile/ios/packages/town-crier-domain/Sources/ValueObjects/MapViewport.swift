/// The visible map rectangle expressed as a WGS84 bounding box (GH#698). The map
/// sends this with an integer slippy zoom so the server returns only the cluster
/// aggregates inside the current view, never the whole zone.
///
/// Edges are decimal degrees: ``west``/``east`` are longitudes, ``south``/
/// ``north`` are latitudes. The data layer renders this as the
/// `bbox=west,south,east,north` query the clusters endpoint expects.
public struct MapViewport: Equatable, Sendable {
  public let west: Double
  public let south: Double
  public let east: Double
  public let north: Double

  public init(west: Double, south: Double, east: Double, north: Double) {
    self.west = west
    self.south = south
    self.east = east
    self.north = north
  }
}
