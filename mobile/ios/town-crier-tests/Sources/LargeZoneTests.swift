import Foundation
import Testing

@testable import TownCrierDomain

@Suite("LargeZone rule")
struct LargeZoneTests {

  @Test func circle_atFreeTierCap_isNotLarge() {
    #expect(LargeZone.isLarge(radiusMetres: 2000) == false)
  }

  @Test func circle_justOverFreeTierCap_isLarge() {
    #expect(LargeZone.isLarge(radiusMetres: 2001) == true)
  }

  @Test func polygon_tenthOfADegreeSquareAt52North_matchesExpectedAreaAndIsLarge() throws {
    let boundary = try square(southLatitude: 51.95, northLatitude: 52.05, westLongitude: -0.05, eastLongitude: 0.05)

    let area = LargeZone.areaSquareMetres(of: boundary)

    #expect(abs(area - 76_120_000) / 76_120_000 < 0.01)
    #expect(LargeZone.isLarge(boundary: boundary) == true)
  }

  @Test func polygon_smallShape_isNotLarge() throws {
    let boundary = try square(southLatitude: 51.500, northLatitude: 51.510, westLongitude: -0.010, eastLongitude: 0.010)

    #expect(LargeZone.isLarge(boundary: boundary) == false)
  }

  @Test func watchZone_circle_usesRadius() throws {
    let small = try WatchZone(name: "A", centre: .cambridge, radiusMetres: 2000)
    let large = try WatchZone(name: "B", centre: .cambridge, radiusMetres: 2001)

    #expect(small.isLarge == false)
    #expect(large.isLarge == true)
  }

  @Test func watchZone_customShape_usesBoundaryAreaNotEnclosingRadius() throws {
    let boundary = try square(southLatitude: 51.500, northLatitude: 51.510, westLongitude: -0.010, eastLongitude: 0.010)
    let zone = try WatchZone(
      name: "Shape", centre: .cambridge, radiusMetres: 50_000, boundary: boundary)

    #expect(zone.isLarge == false)
  }

  private func square(
    southLatitude: Double, northLatitude: Double, westLongitude: Double, eastLongitude: Double
  ) throws -> WatchZoneBoundary {
    try WatchZoneBoundary(vertices: [
      Coordinate(latitude: southLatitude, longitude: westLongitude),
      Coordinate(latitude: southLatitude, longitude: eastLongitude),
      Coordinate(latitude: northLatitude, longitude: eastLongitude),
      Coordinate(latitude: northLatitude, longitude: westLongitude),
    ])
  }
}
