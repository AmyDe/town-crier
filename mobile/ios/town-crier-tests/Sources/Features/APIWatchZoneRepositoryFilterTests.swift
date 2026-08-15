import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// Pre-canned filter (`filterKey`) wire-format tests for
/// `APIWatchZoneRepository` (GH#1098, bead tc-hogkn): create omits an unset
/// filter, update always sends the field explicitly (a value or JSON
/// `null`) -- mirroring `boundary`'s exact create/update split.
@Suite("APIWatchZoneRepository — pre-canned filter")
struct APIWatchZoneRepositoryFilterTests {

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

  private func makeFilteredZone() throws -> WatchZone {
    try WatchZone(
      id: WatchZoneId("zone-filtered"),
      name: "Filtered Area",
      centre: Coordinate(latitude: 51.5074, longitude: -0.1278),
      radiusMetres: 1500,
      filterKey: .loftExtension
    )
  }

  // MARK: - save (create)

  @Test("save sends the filterKey's raw value when set")
  func save_withFilterKey_sendsRawValue() async throws {
    let zone = try makeFilteredZone()
    let (sut, transport) = makeSUT(responses: [
      (Data("{}".utf8), httpResponse(statusCode: 201))
    ])

    try await sut.save(zone)

    let request = try #require(transport.requests.first)
    let body = try #require(request.httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: Any])
    #expect(json["filterKey"] as? String == "loft_extension")
  }

  @Test("save omits the filterKey key when the zone is unfiltered")
  func save_withoutFilterKey_omitsKey() async throws {
    let zone = WatchZone.cambridge
    let (sut, transport) = makeSUT(responses: [
      (Data("{}".utf8), httpResponse(statusCode: 201))
    ])

    try await sut.save(zone)

    let request = try #require(transport.requests.first)
    let body = try #require(request.httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: Any])
    #expect(json["filterKey"] == nil)
  }

  // MARK: - update (patch)

  @Test("update sends the filterKey's raw value when set")
  func update_withFilterKey_sendsRawValue() async throws {
    let zone = try makeFilteredZone()
    let (sut, transport) = makeSUT(responses: [
      (Data("{}".utf8), httpResponse(statusCode: 200))
    ])

    try await sut.update(zone)

    let request = try #require(transport.requests.first)
    let body = try #require(request.httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: Any])
    #expect(json["filterKey"] as? String == "loft_extension")
  }

  @Test("update sends an explicit null filterKey when the zone is unfiltered, never omitted")
  func update_withoutFilterKey_sendsExplicitNull() async throws {
    let zone = WatchZone.cambridge
    let (sut, transport) = makeSUT(responses: [
      (Data("{}".utf8), httpResponse(statusCode: 200))
    ])

    try await sut.update(zone)

    let request = try #require(transport.requests.first)
    let body = try #require(request.httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: Any])
    // The key must be present with an explicit JSON null, not omitted: the
    // server's PATCH tri-state semantics treat an absent key as "leave
    // alone" and only an explicit null as "clear" (GH#1090).
    #expect(json.keys.contains("filterKey"))
    #expect(json["filterKey"] is NSNull)
  }

  // MARK: - loadAll (round trip)

  @Test("loadAll decodes a filtered zone's filterKey from the API response")
  func loadAll_withFilterKey_decodesFilteredZone() async throws {
    let json = """
      {
          "zones": [
              {
                  "id": "zone-filtered",
                  "name": "Filtered Area",
                  "latitude": 51.5074,
                  "longitude": -0.1278,
                  "radiusMetres": 1500,
                  "filterKey": "hmo_house_shares"
              }
          ]
      }
      """
    let (sut, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let zones = try await sut.loadAll()

    #expect(zones.count == 1)
    #expect(zones[0].filterKey == .hmoHouseShares)
  }

  @Test("loadAll decodes an unfiltered zone with filterKey nil (back-compat)")
  func loadAll_withoutFilterKey_decodesUnfilteredZone() async throws {
    let json = """
      {
          "zones": [
              {
                  "id": "zone-001",
                  "name": "CB1 2AD",
                  "latitude": 52.2053,
                  "longitude": 0.1218,
                  "radiusMetres": 2000
              }
          ]
      }
      """
    let (sut, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let zones = try await sut.loadAll()

    #expect(zones.count == 1)
    #expect(zones[0].filterKey == nil)
  }
}
