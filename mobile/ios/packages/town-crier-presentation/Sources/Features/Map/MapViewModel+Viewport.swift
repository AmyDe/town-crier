import Foundation
import TownCrierDomain

extension MapViewModel {
  /// Derives the initial whole-zone viewport and slippy zoom from a zone centre
  /// and radius. Mirrors the camera framing (a span of 2.5x the radius), so the
  /// seeded clusters cover the whole circle until the map view reports its real
  /// visible rect.
  static func initialViewport(
    centre: Coordinate, radiusMetres: Double
  ) -> (viewport: MapViewport, zoom: Int) {
    let metresPerDegreeLat = 111_320.0
    let halfSpanMetres = radiusMetres * 2.5 / 2
    let halfLatDeg = halfSpanMetres / metresPerDegreeLat
    let cosLat = max(0.01, cos(centre.latitude * .pi / 180))
    let halfLonDeg = halfSpanMetres / (metresPerDegreeLat * cosLat)
    let viewport = MapViewport(
      west: centre.longitude - halfLonDeg,
      south: centre.latitude - halfLatDeg,
      east: centre.longitude + halfLonDeg,
      north: centre.latitude + halfLatDeg)
    return (viewport, slippyZoom(forLongitudeSpanDegrees: halfLonDeg * 2))
  }

  /// Standard slippy-map zoom for a longitude span: `log2(360 / span)`, clamped
  /// to the API's accepted 0...20 range.
  static func slippyZoom(forLongitudeSpanDegrees longitudeSpan: Double) -> Int {
    guard longitudeSpan > 0 else { return 20 }
    let zoom = (log2(360.0 / longitudeSpan)).rounded()
    return max(0, min(20, Int(zoom)))
  }
}
