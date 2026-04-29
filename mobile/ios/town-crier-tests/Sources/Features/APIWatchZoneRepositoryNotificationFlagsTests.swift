import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// Per-zone notification flag wire-format tests for `APIWatchZoneRepository` (tc-kh1s).
@Suite("APIWatchZoneRepository — notification flags")
struct APIWatchZoneRepositoryNotificationFlagsTests {

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

  @Test("save sends pushEnabled and emailInstantEnabled in POST body")
  func save_sendsNotificationFlags() async throws {
    let zone = try WatchZone(
      id: WatchZoneId("zone-flags"),
      name: "CB1 2AD",
      centre: Coordinate(latitude: 52.2053, longitude: 0.1218),
      radiusMetres: 2000,
      authorityId: 123,
      pushEnabled: false,
      emailInstantEnabled: true
    )
    let (sut, transport) = makeSUT(responses: [
      (Data("{}".utf8), httpResponse(statusCode: 201))
    ])

    try await sut.save(zone)

    let body = try #require(transport.requests[0].httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: Any])
    #expect(json["pushEnabled"] as? Bool == false)
    #expect(json["emailInstantEnabled"] as? Bool == true)
  }

  @Test("update sends pushEnabled and emailInstantEnabled in PATCH body")
  func update_sendsNotificationFlags() async throws {
    let zone = try WatchZone(
      id: WatchZoneId("zone-flags"),
      name: "CB1 2AD",
      centre: Coordinate(latitude: 52.2053, longitude: 0.1218),
      radiusMetres: 2000,
      authorityId: 123,
      pushEnabled: true,
      emailInstantEnabled: false
    )
    let (sut, transport) = makeSUT(responses: [
      (Data("{}".utf8), httpResponse(statusCode: 200))
    ])

    try await sut.update(zone)

    let body = try #require(transport.requests[0].httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: Any])
    #expect(json["pushEnabled"] as? Bool == true)
    #expect(json["emailInstantEnabled"] as? Bool == false)
  }

  @Test("loadAll maps pushEnabled and emailInstantEnabled to domain")
  func loadAll_mapsNotificationFlags() async throws {
    let json = """
      {
          "zones": [
              {
                  "id": "zone-001",
                  "name": "CB1 2AD",
                  "latitude": 52.2053,
                  "longitude": 0.1218,
                  "radiusMetres": 2000,
                  "authorityId": 123,
                  "pushEnabled": false,
                  "emailInstantEnabled": true
              }
          ]
      }
      """
    let (sut, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let zones = try await sut.loadAll()

    #expect(zones.count == 1)
    #expect(!zones[0].pushEnabled)
    #expect(zones[0].emailInstantEnabled)
  }

  @Test("loadAll defaults pushEnabled and emailInstantEnabled to true when missing from JSON")
  func loadAll_missingFlags_defaultsToTrue() async throws {
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
    #expect(zones[0].pushEnabled)
    #expect(zones[0].emailInstantEnabled)
  }
}
