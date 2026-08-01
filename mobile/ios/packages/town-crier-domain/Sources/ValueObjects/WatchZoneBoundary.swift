import Foundation

/// A closed polygon ring for a custom-shape watch zone, mirroring the
/// server's `watchzones.Boundary` (`api-go/internal/watchzones/zone.go`).
///
/// `WatchZone.boundary == nil` means the zone is a circle; a non-nil,
/// validated `WatchZoneBoundary` means the zone is a custom shape. Once
/// constructed, `vertices` is always a closed ring — its last element equals
/// its first — so `init(vertices:)` is the only supported way to obtain a
/// valid `WatchZoneBoundary`.
public struct WatchZoneBoundary: Equatable, Hashable, Sendable {
  /// The closed ring: the last vertex always equals the first.
  public let vertices: [Coordinate]

  /// minVertices and maxVertices bound the ring size, excluding the closing
  /// repeat: 3 is the geometric minimum for a polygon, 50 is generous for a
  /// hand-drawn shape while bounding payload size and matching-query cost.
  private static let minVertices = 3
  private static let maxVertices = 50

  /// A generous bounding box around Great Britain and Northern Ireland, used
  /// only to reject vertices that are obviously not the UK — not a precise
  /// coastline check. Mirrors the server's uk* bounds constants.
  private static let ukMinLatitude = 49.5
  private static let ukMaxLatitude = 61.0
  private static let ukMinLongitude = -8.5
  private static let ukMaxLongitude = 2.0

  /// Validates and normalises raw vertices into a closed polygon ring: 3-50
  /// distinct vertices (after auto-closing, not counting the closing
  /// repeat), an open ring is auto-closed by appending a copy of the first
  /// vertex (a ring that already closes is left as-is), all vertices within
  /// UK bounds, and the ring must be simple (no two non-adjacent edges
  /// cross). Mirrors the server's `NewBoundary`.
  public init(vertices rawVertices: [Coordinate]) throws {
    guard !rawVertices.isEmpty else {
      throw DomainError.invalidWatchZoneBoundaryVertexCount
    }
    var ring = rawVertices
    if ring[0] != ring[ring.count - 1] {
      ring.append(ring[0])
    }
    let distinct = ring.dropLast()
    guard distinct.count >= Self.minVertices, distinct.count <= Self.maxVertices else {
      throw DomainError.invalidWatchZoneBoundaryVertexCount
    }
    var seen: Set<Coordinate> = []
    for vertex in distinct {
      guard seen.insert(vertex).inserted else {
        throw DomainError.invalidWatchZoneBoundaryDuplicateVertex
      }
    }
    for vertex in distinct {
      guard (Self.ukMinLatitude...Self.ukMaxLatitude).contains(vertex.latitude),
        (Self.ukMinLongitude...Self.ukMaxLongitude).contains(vertex.longitude)
      else {
        throw DomainError.invalidWatchZoneBoundaryOutOfBounds
      }
    }
    guard Self.signedArea(ring) != 0 else {
      // A collinear ring has no interior, so no point can ever fall inside
      // it; treat it as a degenerate (self-touching) shape.
      throw DomainError.invalidWatchZoneBoundarySelfIntersecting
    }
    guard !Self.selfIntersects(ring) else {
      throw DomainError.invalidWatchZoneBoundarySelfIntersecting
    }
    self.vertices = ring
  }

  /// The polygon centroid (area-weighted, shoelace formula) — not the
  /// arithmetic mean of the vertices, which differs from the true centroid
  /// for any non-uniform shape (e.g. an L-shape). Longitude and latitude are
  /// treated as planar x/y, an acceptable approximation at the
  /// sub-tens-of-km scale a hand-drawn watch zone can reach. Mirrors the
  /// server's `Boundary.Centroid()`.
  public var centroid: (latitude: Double, longitude: Double) {
    let edgeCount = vertices.count - 1
    var area = 0.0
    var centreX = 0.0
    var centreY = 0.0
    for index in 0..<edgeCount {
      let vertex = vertices[index]
      let next = vertices[index + 1]
      let cross = vertex.longitude * next.latitude - next.longitude * vertex.latitude
      area += cross
      centreX += (vertex.longitude + next.longitude) * cross
      centreY += (vertex.latitude + next.latitude) * cross
    }
    area /= 2
    centreX /= 6 * area
    centreY /= 6 * area
    return (latitude: centreY, longitude: centreX)
  }

  /// The great-circle distance in metres from `centroid` to the furthest
  /// vertex — the radius of the smallest centroid-centred circle that fully
  /// encloses the shape. Mirrors the server's `Boundary.EnclosingRadiusMetres()`,
  /// and what a custom-shape `WatchZone` derives into its `radiusMetres`
  /// field so every read path built for circles keeps working unchanged.
  public var enclosingRadiusMetres: Double {
    let centre = centroid
    var maxDistance = 0.0
    for vertex in vertices.dropLast() {
      let distance = Self.haversineMetres(
        latitude1: centre.latitude,
        longitude1: centre.longitude,
        latitude2: vertex.latitude,
        longitude2: vertex.longitude
      )
      maxDistance = max(maxDistance, distance)
    }
    return maxDistance
  }

  // MARK: - Geometry helpers

  /// Twice the signed shoelace area of the closed ring. Zero only for a
  /// degenerate (zero-area, e.g. collinear) ring.
  private static func signedArea(_ ring: [Coordinate]) -> Double {
    let edgeCount = ring.count - 1
    guard edgeCount >= 1 else { return 0 }
    var sum = 0.0
    for index in 0..<edgeCount {
      sum +=
        ring[index].longitude * ring[index + 1].latitude
        - ring[index + 1].longitude * ring[index].latitude
    }
    return sum
  }

  /// Whether any two non-adjacent edges of the closed ring cross. O(n²) over
  /// the edges; n is capped at 50 vertices by `init`, so this is cheap and a
  /// sweep-line algorithm is not warranted.
  private static func selfIntersects(_ ring: [Coordinate]) -> Bool {
    let edgeCount = ring.count - 1
    guard edgeCount >= 3 else { return false }
    for first in 0..<edgeCount {
      let firstStart = ring[first]
      let firstEnd = ring[first + 1]
      guard first + 2 <= edgeCount else { continue }
      for second in (first + 2)..<edgeCount {
        if first == 0 && second == edgeCount - 1 {
          // The last edge and the first edge share vertex ring[0]==ring[edgeCount]:
          // adjacent via the ring's wraparound, not a crossing.
          continue
        }
        let secondStart = ring[second]
        let secondEnd = ring[second + 1]
        if segmentsIntersect(firstStart, firstEnd, secondStart, secondEnd) {
          return true
        }
      }
    }
    return false
  }

  /// Whether segments pt1-pt2 and pt3-pt4 intersect, including one touching
  /// the other at an endpoint or lying collinearly on it. Standard
  /// orientation-based test (see e.g. CLRS §33.1).
  private static func segmentsIntersect(
    _ pt1: Coordinate,
    _ pt2: Coordinate,
    _ pt3: Coordinate,
    _ pt4: Coordinate
  ) -> Bool {
    let ori1 = orientation(pt3, pt4, pt1)
    let ori2 = orientation(pt3, pt4, pt2)
    let ori3 = orientation(pt1, pt2, pt3)
    let ori4 = orientation(pt1, pt2, pt4)

    let straddlesFirstEdge = (ori1 > 0 && ori2 < 0) || (ori1 < 0 && ori2 > 0)
    let straddlesSecondEdge = (ori3 > 0 && ori4 < 0) || (ori3 < 0 && ori4 > 0)
    if straddlesFirstEdge && straddlesSecondEdge {
      return true
    }
    if ori1 == 0 && onSegment(pt3, pt4, pt1) { return true }
    if ori2 == 0 && onSegment(pt3, pt4, pt2) { return true }
    if ori3 == 0 && onSegment(pt1, pt2, pt3) { return true }
    if ori4 == 0 && onSegment(pt1, pt2, pt4) { return true }
    return false
  }

  /// Twice the signed area of triangle origin,next,point: positive for
  /// counter-clockwise, negative for clockwise, zero when the three points
  /// are collinear.
  private static func orientation(_ origin: Coordinate, _ next: Coordinate, _ point: Coordinate) -> Double {
    (next.longitude - origin.longitude) * (point.latitude - origin.latitude)
      - (next.latitude - origin.latitude) * (point.longitude - origin.longitude)
  }

  /// Whether `point`, already known to be collinear with segment
  /// `segmentStart`-`segmentEnd`, lies within its bounding box — i.e.
  /// actually on the segment rather than on the line through it extended
  /// beyond either endpoint.
  private static func onSegment(_ segmentStart: Coordinate, _ segmentEnd: Coordinate, _ point: Coordinate) -> Bool {
    point.longitude >= min(segmentStart.longitude, segmentEnd.longitude)
      && point.longitude <= max(segmentStart.longitude, segmentEnd.longitude)
      && point.latitude >= min(segmentStart.latitude, segmentEnd.latitude)
      && point.latitude <= max(segmentStart.latitude, segmentEnd.latitude)
  }

  /// Great-circle distance between two lat/lon points, in metres.
  private static func haversineMetres(
    latitude1: Double,
    longitude1: Double,
    latitude2: Double,
    longitude2: Double
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
