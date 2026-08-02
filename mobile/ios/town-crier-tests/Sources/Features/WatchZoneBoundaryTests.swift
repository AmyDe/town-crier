import Foundation
import Testing

@testable import TownCrierDomain

/// Mirrors `api-go/internal/watchzones/zone.go`'s `NewBoundary`/`Boundary`
/// validation and geometry (GH#1031, bead tc-6he3x.7).
@Suite("WatchZoneBoundary value object")
struct WatchZoneBoundaryTests {

  // MARK: - Closure

  @Test("an open ring is auto-closed by appending a copy of the first vertex")
  func init_openRing_autoCloses() throws {
    let triangle = try [
      Coordinate(latitude: 51.50, longitude: -0.10),
      Coordinate(latitude: 51.51, longitude: -0.09),
      Coordinate(latitude: 51.50, longitude: -0.09),
    ]

    let boundary = try WatchZoneBoundary(vertices: triangle)

    #expect(boundary.vertices.count == triangle.count + 1)
    #expect(boundary.vertices.first == boundary.vertices.last)
  }

  @Test("an already-closed ring is not doubled")
  func init_alreadyClosedRing_isNotDoubled() throws {
    let closed = try [
      Coordinate(latitude: 51.50, longitude: -0.10),
      Coordinate(latitude: 51.51, longitude: -0.09),
      Coordinate(latitude: 51.50, longitude: -0.09),
      Coordinate(latitude: 51.50, longitude: -0.10),
    ]

    let boundary = try WatchZoneBoundary(vertices: closed)

    #expect(boundary.vertices.count == closed.count)
  }

  // MARK: - Vertex count

  @Test("fewer than 3 distinct vertices is rejected")
  func init_fewerThanThreeVertices_throws() throws {
    let vertices = try [
      Coordinate(latitude: 51.50, longitude: -0.10),
      Coordinate(latitude: 51.51, longitude: -0.11),
    ]

    #expect(throws: DomainError.invalidWatchZoneBoundaryVertexCount) {
      try WatchZoneBoundary(vertices: vertices)
    }
  }

  @Test("more than 50 distinct vertices is rejected")
  func init_moreThanFiftyVertices_throws() throws {
    var vertices: [Coordinate] = []
    for index in 0..<51 {
      vertices.append(
        try Coordinate(latitude: 51.5, longitude: -0.5 + Double(index) * 0.001)
      )
    }

    #expect(throws: DomainError.invalidWatchZoneBoundaryVertexCount) {
      try WatchZoneBoundary(vertices: vertices)
    }
  }

  @Test("exactly 50 distinct vertices is accepted")
  func init_exactlyFiftyVertices_succeeds() throws {
    let centreLat = 51.5074
    let centreLon = -0.1278
    let radiusDeg = 0.05
    let count = 50
    var vertices: [Coordinate] = []
    for index in 0..<count {
      let angle = 2 * Double.pi * Double(index) / Double(count)
      vertices.append(
        try Coordinate(
          latitude: centreLat + radiusDeg * sin(angle),
          longitude: centreLon + radiusDeg * cos(angle)
        )
      )
    }

    let boundary = try WatchZoneBoundary(vertices: vertices)

    #expect(boundary.vertices.count == count + 1)
  }

  @Test("a duplicate interior vertex is rejected")
  func init_duplicateVertex_throws() throws {
    let vertices = try [
      Coordinate(latitude: 51.50, longitude: -0.10),
      Coordinate(latitude: 51.51, longitude: -0.09),
      Coordinate(latitude: 51.51, longitude: -0.09),
      Coordinate(latitude: 51.50, longitude: -0.08),
    ]

    #expect(throws: DomainError.invalidWatchZoneBoundaryDuplicateVertex) {
      try WatchZoneBoundary(vertices: vertices)
    }
  }

  // MARK: - Self-intersection

  @Test("a self-intersecting (bowtie) ring is rejected")
  func init_selfIntersectingRing_throws() throws {
    let bowtie = try [
      Coordinate(latitude: 51.50, longitude: -0.10),
      Coordinate(latitude: 51.51, longitude: -0.09),
      Coordinate(latitude: 51.50, longitude: -0.09),
      Coordinate(latitude: 51.51, longitude: -0.10),
    ]

    #expect(throws: DomainError.invalidWatchZoneBoundarySelfIntersecting) {
      try WatchZoneBoundary(vertices: bowtie)
    }
  }

  @Test("a zero-area (collinear) ring is rejected")
  func init_collinearRing_throws() throws {
    let collinear = try [
      Coordinate(latitude: 51.50, longitude: -0.10),
      Coordinate(latitude: 51.50, longitude: -0.09),
      Coordinate(latitude: 51.50, longitude: -0.08),
    ]

    #expect(throws: DomainError.invalidWatchZoneBoundarySelfIntersecting) {
      try WatchZoneBoundary(vertices: collinear)
    }
  }

  // MARK: - Out of bounds

  @Test("a vertex outside the UK bounding box is rejected")
  func init_vertexOutsideUKBounds_throws() throws {
    let vertices = try [
      Coordinate(latitude: 51.5, longitude: -0.1),
      Coordinate(latitude: 51.51, longitude: -0.11),
      Coordinate(latitude: 51.5, longitude: 40.0),
    ]

    #expect(throws: DomainError.invalidWatchZoneBoundaryOutOfBounds) {
      try WatchZoneBoundary(vertices: vertices)
    }
  }

  // MARK: - Centroid

  @Test("centroid of a known rectangle is its geometric centre")
  func centroid_knownRectangle_isGeometricCentre() throws {
    let rectangle = try [
      Coordinate(latitude: 51, longitude: -1),
      Coordinate(latitude: 52, longitude: -1),
      Coordinate(latitude: 52, longitude: 0),
      Coordinate(latitude: 51, longitude: 0),
    ]
    let boundary = try WatchZoneBoundary(vertices: rectangle)

    let centroid = boundary.centroid

    #expect(abs(centroid.latitude - 51.5) < 1e-9)
    #expect(abs(centroid.longitude - (-0.5)) < 1e-9)
  }

  @Test("centroid is the area-weighted centre, not the vertex average")
  func centroid_asymmetricPolygon_isNotVertexAverage() throws {
    // A staircase/L-shape: (lat,lon) (0,0),(0,6),(2,6),(2,2),(4,2),(4,0).
    // True polygon centroid is (1.5, 2.5); the naive vertex average is
    // (2.0, 2.667) — different enough to fail a naive-average implementation.
    // Shifted into UK bounds by a +51 latitude, -7 longitude offset (the
    // shape spans 6 degrees of longitude, so the offset must keep both ends
    // within ukMinLongitude/ukMaxLongitude, not just the starting vertex).
    let baseLat = 51.0
    let baseLon = -7.0
    let shape = try [
      Coordinate(latitude: baseLat, longitude: baseLon),
      Coordinate(latitude: baseLat, longitude: baseLon + 6),
      Coordinate(latitude: baseLat + 2, longitude: baseLon + 6),
      Coordinate(latitude: baseLat + 2, longitude: baseLon + 2),
      Coordinate(latitude: baseLat + 4, longitude: baseLon + 2),
      Coordinate(latitude: baseLat + 4, longitude: baseLon),
    ]
    let boundary = try WatchZoneBoundary(vertices: shape)

    let centroid = boundary.centroid

    #expect(abs(centroid.latitude - (baseLat + 1.5)) < 1e-9)
    #expect(abs(centroid.longitude - (baseLon + 2.5)) < 1e-9)
  }

  // MARK: - Enclosing radius

  @Test("enclosing radius of a known square matches the precomputed distance")
  func enclosingRadiusMetres_knownSquare_matchesPrecomputedDistance() throws {
    let centreLat = 51.5074
    let centreLon = -0.1278
    let deltaLat = 0.0089831
    let deltaLon = 0.0144328
    let square = try [
      Coordinate(latitude: centreLat + deltaLat, longitude: centreLon + deltaLon),
      Coordinate(latitude: centreLat + deltaLat, longitude: centreLon - deltaLon),
      Coordinate(latitude: centreLat - deltaLat, longitude: centreLon - deltaLon),
      Coordinate(latitude: centreLat - deltaLat, longitude: centreLon + deltaLon),
    ]
    let boundary = try WatchZoneBoundary(vertices: square)

    let radius = boundary.enclosingRadiusMetres

    #expect(abs(radius - 1_412.694_247_415_168) < 0.5)
  }
}
