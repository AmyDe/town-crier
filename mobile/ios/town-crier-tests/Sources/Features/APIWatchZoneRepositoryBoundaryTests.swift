import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// Custom-shape boundary wire-format tests for `APIWatchZoneRepository`
/// (GH#1031, bead tc-6he3x.7): the GeoJSON Polygon shape is mirrored exactly
/// against the server (`api-go/internal/watchzones/handler.go`).
@Suite("APIWatchZoneRepository — custom-shape boundary")
struct APIWatchZoneRepositoryBoundaryTests {

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APIWatchZoneRepository, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIWatchZoneRepository(apiClient: apiClient)
    return (sut, transport)
  }

  // swiftlint:disable force_unwrapping
  private func httpResponse(statusCode: Int) -> HTTPURLResponse {
    HTTPURLResponse(
      url: baseURL,
      statusCode: statusCode,
      httpVersion: nil,
      headerFields: nil
    )!
  }
  // swiftlint:enable force_unwrapping

  private func makeCustomShapeZone() throws -> WatchZone {
    let boundary = try WatchZoneBoundary(vertices: [
      Coordinate(latitude: 51.50, longitude: -0.10),
      Coordinate(latitude: 51.51, longitude: -0.09),
      Coordinate(latitude: 51.50, longitude: -0.09),
    ])
    return try WatchZone(
      id: WatchZoneId("zone-custom"),
      name: "Custom Area",
      centre: Coordinate(latitude: 51.5074, longitude: -0.1278),
      radiusMetres: 1500,
      authorityId: 123,
      boundary: boundary
    )
  }

  // MARK: - save (create)

  @Test("save sends the boundary as a GeoJSON Polygon, first vertex repeated last")
  func save_withBoundary_sendsGeoJSONPolygon() async throws {
    let zone = try makeCustomShapeZone()
    let (sut, transport) = makeSUT(responses: [
      (Data("{}".utf8), httpResponse(statusCode: 201))
    ])

    try await sut.save(zone)

    let body = try #require(transport.requests[0].httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: Any])
    let boundaryJSON = try #require(json["boundary"] as? [String: Any])
    #expect(boundaryJSON["type"] as? String == "Polygon")
    let coordinates = try #require(boundaryJSON["coordinates"] as? [[[Double]]])
    #expect(coordinates.count == 1)
    #expect(coordinates[0].count == 4)
    #expect(coordinates[0].first == coordinates[0].last)
    #expect(coordinates[0][0] == [-0.10, 51.50])
  }

  @Test("save omits the boundary key when the zone is a circle")
  func save_withoutBoundary_omitsBoundaryKey() async throws {
    let zone = WatchZone.cambridge
    let (sut, transport) = makeSUT(responses: [
      (Data("{}".utf8), httpResponse(statusCode: 201))
    ])

    try await sut.save(zone)

    let body = try #require(transport.requests[0].httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: Any])
    #expect(json["boundary"] == nil)
  }

  // MARK: - update (patch)

  @Test("update sends the boundary as a GeoJSON Polygon when present")
  func update_withBoundary_sendsGeoJSONPolygon() async throws {
    let zone = try makeCustomShapeZone()
    let (sut, transport) = makeSUT(responses: [
      (Data("{}".utf8), httpResponse(statusCode: 200))
    ])

    try await sut.update(zone)

    let body = try #require(transport.requests[0].httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: Any])
    let boundaryJSON = try #require(json["boundary"] as? [String: Any])
    #expect(boundaryJSON["type"] as? String == "Polygon")
  }

  @Test("update omits the boundary key when the zone is a circle")
  func update_withoutBoundary_omitsBoundaryKey() async throws {
    let zone = WatchZone.cambridge
    let (sut, transport) = makeSUT(responses: [
      (Data("{}".utf8), httpResponse(statusCode: 200))
    ])

    try await sut.update(zone)

    let body = try #require(transport.requests[0].httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: Any])
    #expect(json["boundary"] == nil)
  }

  // MARK: - loadAll (round trip)

  @Test("loadAll decodes a custom-shape zone's boundary from the API response")
  func loadAll_withBoundary_decodesCustomShapeZone() async throws {
    let json = """
      {
          "zones": [
              {
                  "id": "zone-custom",
                  "name": "Custom Area",
                  "latitude": 51.5074,
                  "longitude": -0.1278,
                  "radiusMetres": 1500,
                  "authorityId": 123,
                  "boundary": {
                      "type": "Polygon",
                      "coordinates": [[
                          [-0.10, 51.50],
                          [-0.09, 51.51],
                          [-0.09, 51.50],
                          [-0.10, 51.50]
                      ]]
                  }
              }
          ]
      }
      """
    let (sut, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let zones = try await sut.loadAll()

    #expect(zones.count == 1)
    #expect(zones[0].isCustomShape)
    #expect(zones[0].boundary?.vertices.count == 4)
  }

  @Test("loadAll decodes a circle zone with boundary nil (back-compat)")
  func loadAll_withoutBoundary_decodesCircleZone() async throws {
    let json = """
      {
          "zones": [
              {
                  "id": "zone-001",
                  "name": "CB1 2AD",
                  "latitude": 52.2053,
                  "longitude": 0.1218,
                  "radiusMetres": 2000,
                  "authorityId": 123
              }
          ]
      }
      """
    let (sut, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let zones = try await sut.loadAll()

    #expect(zones.count == 1)
    #expect(!zones[0].isCustomShape)
    #expect(zones[0].boundary == nil)
  }
}
