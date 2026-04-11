import MapKit
import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ZoneMapPreview")
@MainActor
struct ZoneMapPreviewTests {

  // MARK: - Initialization

  @Test func init_withCentreAndRadius_createsView() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let sut = ZoneMapPreview(centre: centre, radiusMetres: 1000)
    _ = sut.body
  }

  @Test func init_withDefaultStrokeWidth_createsView() throws {
    let centre = try Coordinate(latitude: 51.5074, longitude: -0.1278)
    let sut = ZoneMapPreview(centre: centre, radiusMetres: 500)
    #expect(sut.strokeWidth == 1)
  }

  @Test func init_withCustomStrokeWidth_createsView() throws {
    let centre = try Coordinate(latitude: 51.5074, longitude: -0.1278)
    let sut = ZoneMapPreview(centre: centre, radiusMetres: 2000, strokeWidth: 2)
    #expect(sut.strokeWidth == 2)
  }
}
