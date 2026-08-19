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
      filterKey: "loft_extension"
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
    #expect(zones[0].filterKey == "hmo_house_shares")
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

  /// GH#1104/tc-m8j90.2's core fix for tc-8z9ri: a key this build's fetched
  /// catalog doesn't recognise no longer fails open to `nil` at decode time
  /// (the old `WatchZoneFilterKey(rawValue:)` conversion did) -- it
  /// round-trips verbatim as an opaque string, same as any other key.
  @Test("loadAll decodes an unrecognised filterKey string verbatim, not nil")
  func loadAll_unrecognisedFilterKey_decodesVerbatim() async throws {
    let json = """
      {
          "zones": [
              {
                  "id": "zone-future-filter",
                  "name": "Filtered Area",
                  "latitude": 51.5074,
                  "longitude": -0.1278,
                  "radiusMetres": 1500,
                  "filterKey": "a_filter_this_build_predates"
              }
          ]
      }
      """
    let (sut, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let zones = try await sut.loadAll()

    #expect(zones.count == 1)
    #expect(zones[0].filterKey == "a_filter_this_build_predates")
  }

  // MARK: - filterCatalog (GH#1104, tc-m8j90.2)

  @Test("filterCatalog sends GET /v1/watch-zones/filter-catalog and maps the response envelope")
  func filterCatalog_decodesResponseEnvelope() async throws {
    let json = """
      {
          "filters": [
              {"key": "fewer_notifications", "displayName": "Fewer notifications", "description": "Cuts out noise."},
              {"key": "loft_extension", "displayName": "Loft extension", "description": "Loft conversions."}
          ]
      }
      """
    let (sut, transport) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let entries = try await sut.filterCatalog()

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    #expect(request.url?.path().contains("/v1/watch-zones/filter-catalog") == true)

    let expected = [
      FilterCatalogEntry(
        key: "fewer_notifications",
        displayName: "Fewer notifications",
        description: "Cuts out noise."
      ),
      FilterCatalogEntry(
        key: "loft_extension",
        displayName: "Loft extension",
        description: "Loft conversions."
      ),
    ]
    #expect(entries == expected)
  }

  @Test("filterCatalog returns an empty array for an empty filters array")
  func filterCatalog_emptyFilters_returnsEmptyArray() async throws {
    let json = #"{"filters": []}"#
    let (sut, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let entries = try await sut.filterCatalog()

    #expect(entries.isEmpty)
  }

  @Test("filterCatalog with network error throws networkUnavailable")
  func filterCatalog_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL, authService: authService, transport: transport)
    let sut = APIWatchZoneRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.filterCatalog()
    }
  }

  @Test("filterCatalog with a malformed response surfaces a decode error the ViewModel can catch")
  func filterCatalog_malformedResponse_throwsError() async {
    let (sut, _) = makeSUT(responses: [
      (Data("not json".utf8), httpResponse(statusCode: 200))
    ])

    await #expect(throws: (any Error).self) {
      _ = try await sut.filterCatalog()
    }
  }
}
