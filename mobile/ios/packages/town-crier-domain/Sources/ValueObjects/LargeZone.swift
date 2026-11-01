import Foundation

/// The single rule for when a watch zone counts as "large": its area is
/// strictly greater than that of a 2 km circle. The comparison is strict so a
/// free-tier user pinned at the 2 km cap (see ``WatchZoneLimits``) never sees
/// the large-zone warning.
public enum LargeZone {
  /// Area of a 2 km circle, in square metres.
  public static let areaThresholdSquareMetres: Double = .pi * 2000 * 2000

  private static let earthRadiusMetres = 6_371_008.8

  public static func isLarge(radiusMetres: Double) -> Bool {
    .pi * radiusMetres * radiusMetres > areaThresholdSquareMetres
  }

  public static func isLarge(boundary: WatchZoneBoundary) -> Bool {
    areaSquareMetres(of: boundary) > areaThresholdSquareMetres
  }

  /// Polygon area in square metres via the shoelace formula on an
  /// equirectangular projection about the ring's mean latitude. Accurate to
  /// about 1% at UK scale.
  public static func areaSquareMetres(of boundary: WatchZoneBoundary) -> Double {
    let ring = boundary.vertices
    let distinct = ring.dropLast()
    guard !distinct.isEmpty else { return 0 }
    let meanLatitude = distinct.map(\.latitude).reduce(0, +) / Double(distinct.count)
    let xScale = cos(meanLatitude * .pi / 180) * earthRadiusMetres * .pi / 180
    let yScale = earthRadiusMetres * .pi / 180
    var twiceArea = 0.0
    for index in 0..<(ring.count - 1) {
      let current = ring[index]
      let next = ring[index + 1]
      twiceArea +=
        (current.longitude * xScale) * (next.latitude * yScale)
        - (next.longitude * xScale) * (current.latitude * yScale)
    }
    return abs(twiceArea) / 2
  }
}
