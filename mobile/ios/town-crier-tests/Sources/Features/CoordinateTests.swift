import Testing

@testable import TownCrierDomain

@Suite("Coordinate value object")
struct CoordinateTests {
  @Test func init_validValues_createsCoordinate() throws {
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    #expect(coordinate.latitude == 52.2053)
    #expect(coordinate.longitude == 0.1218)
  }

  @Test func init_latitudeTooHigh_throws() {
    #expect(throws: DomainError.invalidCoordinate) {
      try Coordinate(latitude: 91.0, longitude: 0.0)
    }
  }

  @Test func init_latitudeTooLow_throws() {
    #expect(throws: DomainError.invalidCoordinate) {
      try Coordinate(latitude: -91.0, longitude: 0.0)
    }
  }

  @Test func init_longitudeTooHigh_throws() {
    #expect(throws: DomainError.invalidCoordinate) {
      try Coordinate(latitude: 0.0, longitude: 181.0)
    }
  }

  @Test func init_longitudeTooLow_throws() {
    #expect(throws: DomainError.invalidCoordinate) {
      try Coordinate(latitude: 0.0, longitude: -181.0)
    }
  }

  @Test func equality_sameValues_areEqual() throws {
    let a = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let b = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    #expect(a == b)
  }

  // MARK: - distanceMetres

  @Test func distanceMetres_sameCoordinate_isZero() throws {
    let a = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    #expect(a.distanceMetres(to: a) == 0)
  }

  /// Antipodal points on the equator: haversine's `h` term computes to
  /// exactly 1.0, the boundary this test exists to cover. Regression for a
  /// CodeRabbit-flagged bug where `1 - h` could go negative and `sqrt`
  /// return NaN.
  @Test func distanceMetres_antipodalOnEquator_isHalfEarthCircumference() throws {
    let a = try Coordinate(latitude: 0, longitude: 0)
    let b = try Coordinate(latitude: 0, longitude: 180)

    let distance = a.distanceMetres(to: b)

    #expect(distance.isFinite)
    #expect(abs(distance - .pi * 6_371_000) < 1.0)
  }

  /// A near-antipodal pair found by brute-force search over random
  /// near-antipodal inputs to push the raw (pre-clamp) haversine `h` term
  /// fractionally above 1.0 in double precision (`1.0000000000000004` in
  /// Python's `math`, same IEEE 754 double arithmetic as Swift's `Double`).
  /// Without the clamp, `1 - h` goes negative and `sqrt` returns NaN, which
  /// then poisons `atan2` and every caller of `distanceMetres`/`contains(_:)`.
  @Test func distanceMetres_nearAntipodalPair_isFiniteNotNaN() throws {
    let a = try Coordinate(latitude: -58.97018354817475, longitude: -97.36776415155397)
    let b = try Coordinate(latitude: 58.970183085348935, longitude: 82.6322355333763)

    let distance = a.distanceMetres(to: b)

    #expect(distance.isFinite)
    #expect(abs(distance - .pi * 6_371_000) < 10.0)
  }
}
