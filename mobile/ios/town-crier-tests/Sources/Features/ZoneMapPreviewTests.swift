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

  // MARK: - Custom-shape boundary (tc-7se1w.1)

  @Test func init_withoutBoundaryVertices_defaultsToNil() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let sut = ZoneMapPreview(centre: centre, radiusMetres: 1000)
    #expect(sut.boundaryVertices == nil)
  }

  @Test func init_withBoundaryVertices_storesVertices() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let vertices = try [
      Coordinate(latitude: 52.20, longitude: 0.12),
      Coordinate(latitude: 52.21, longitude: 0.13),
      Coordinate(latitude: 52.20, longitude: 0.13),
    ]
    let sut = ZoneMapPreview(centre: centre, radiusMetres: 1000, boundaryVertices: vertices)
    #expect(sut.boundaryVertices == vertices)
  }

  @Test func body_withBoundaryVertices_createsView() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let vertices = try [
      Coordinate(latitude: 52.20, longitude: 0.12),
      Coordinate(latitude: 52.21, longitude: 0.13),
      Coordinate(latitude: 52.20, longitude: 0.13),
    ]
    let sut = ZoneMapPreview(centre: centre, radiusMetres: 1000, boundaryVertices: vertices)
    _ = sut.body
  }

  @Test func isCustomShape_withThreeOrMoreVertices_isTrue() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let vertices = try [
      Coordinate(latitude: 52.20, longitude: 0.12),
      Coordinate(latitude: 52.21, longitude: 0.13),
      Coordinate(latitude: 52.20, longitude: 0.13),
    ]
    let sut = ZoneMapPreview(centre: centre, radiusMetres: 1000, boundaryVertices: vertices)
    #expect(sut.isCustomShape)
  }

  @Test func isCustomShape_withNilVertices_isFalse() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let sut = ZoneMapPreview(centre: centre, radiusMetres: 1000)
    #expect(!sut.isCustomShape)
  }

  @Test func isCustomShape_withFewerThanThreeVertices_isFalse() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let vertices = try [
      Coordinate(latitude: 52.20, longitude: 0.12),
      Coordinate(latitude: 52.21, longitude: 0.13),
    ]
    let sut = ZoneMapPreview(centre: centre, radiusMetres: 1000, boundaryVertices: vertices)
    #expect(!sut.isCustomShape)
  }
}
