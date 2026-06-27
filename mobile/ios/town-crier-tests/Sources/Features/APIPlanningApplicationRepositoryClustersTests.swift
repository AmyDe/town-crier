import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// Covers the server-side map clustering fetch (GH#698): it sends
/// `?bbox=west,south,east,north&zoom=<int>` plus an optional `?status=`, decodes
/// the bare `[MapClusterDTO]` body, and maps each cell — including the
/// single-member case that carries `{authority, name}` — into a `MapCluster`.
@Suite("APIPlanningApplicationRepository — clusters fetch")
struct APIPlanningApplicationRepositoryClustersTests {
  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  // swiftlint:disable:next force_try
  private let testZone = try! WatchZone(
    id: WatchZoneId("zone-123"),
    name: "Cambridge",
    centre: Coordinate(latitude: 52.2053, longitude: 0.1218),
    radiusMetres: 2000,
    authorityId: 123
  )

  private let viewport = MapViewport(west: -0.2, south: 51.4, east: 0.0, north: 51.6)

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APIPlanningApplicationRepository, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    return (APIPlanningApplicationRepository(apiClient: apiClient), transport)
  }

  private func httpResponse(statusCode: Int, headers: [String: String]) -> HTTPURLResponse {
    // swiftlint:disable:next force_unwrapping
    HTTPURLResponse(url: baseURL, statusCode: statusCode, httpVersion: nil, headerFields: headers)!
  }

  private func queryItems(_ transport: StubHTTPTransport, at index: Int) throws -> [URLQueryItem] {
    let url = try #require(transport.requests[index].url)
    let components = URLComponents(url: url, resolvingAgainstBaseURL: false)
    return components?.queryItems ?? []
  }

  private static let clustersJSON = """
    [
      {
        "latitude": 51.51,
        "longitude": -0.12,
        "count": 194,
        "statusCounts": { "Permitted": 120, "Undecided": 60, "Rejected": 14 },
        "applicationId": null
      },
      {
        "latitude": 51.515,
        "longitude": -0.13,
        "count": 1,
        "statusCounts": { "Permitted": 1 },
        "applicationId": { "authority": "123", "name": "2026/0042" }
      }
    ]
    """

  @Test("sends bbox and zoom and omits status for the All filter")
  func fetchClusters_allFilter_sendsBboxAndZoomOnly() async throws {
    let (sut, transport) = makeSUT(responses: [
      (Data("[]".utf8), httpResponse(statusCode: 200, headers: [:]))
    ])

    _ = try await sut.fetchClusters(for: testZone, viewport: viewport, zoom: 13, filter: .all)

    let url = try #require(transport.requests[0].url)
    #expect(url.path() == "/v1/me/watch-zones/zone-123/applications/clusters")
    let items = try queryItems(transport, at: 0)
    #expect(items.contains(URLQueryItem(name: "bbox", value: "-0.2,51.4,0.0,51.6")))
    #expect(items.contains(URLQueryItem(name: "zoom", value: "13")))
    #expect(!items.contains { $0.name == "status" })
  }

  @Test("a status filter sends ?status=<appState>")
  func fetchClusters_statusFilter_sendsStatusParam() async throws {
    let (sut, transport) = makeSUT(responses: [
      (Data("[]".utf8), httpResponse(statusCode: 200, headers: [:]))
    ])

    _ = try await sut.fetchClusters(
      for: testZone, viewport: viewport, zoom: 9, filter: .status(.rejected))

    let items = try queryItems(transport, at: 0)
    #expect(items.contains(URLQueryItem(name: "status", value: "Rejected")))
    #expect(items.contains(URLQueryItem(name: "zoom", value: "9")))
  }

  @Test("decodes the cluster array including the single-member case")
  func fetchClusters_decodesMultiAndSingleMemberCells() async throws {
    let (sut, _) = makeSUT(responses: [
      (Data(Self.clustersJSON.utf8), httpResponse(statusCode: 200, headers: [:]))
    ])

    let clusters = try await sut.fetchClusters(
      for: testZone, viewport: viewport, zoom: 11, filter: .all)

    #expect(clusters.count == 2)

    let bubble = try #require(clusters.first { $0.count > 1 })
    #expect(bubble.member == nil)
    #expect(bubble.statusCounts[.permitted] == 120)
    #expect(bubble.statusCounts[.undecided] == 60)
    #expect(bubble.statusCounts[.rejected] == 14)
    #expect(bubble.coordinate.latitude == 51.51)
    #expect(bubble.coordinate.longitude == -0.12)

    let single = try #require(clusters.first { $0.count == 1 })
    #expect(single.member == PlanningApplicationId(authority: "123", name: "2026/0042"))
    #expect(single.memberStatus == .permitted)
  }

  @Test("maps a network error to DomainError.networkUnavailable")
  func fetchClusters_networkError_mapsToDomainError() async throws {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL, authService: authService, transport: transport)
    let sut = APIPlanningApplicationRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.fetchClusters(for: testZone, viewport: viewport, zoom: 10, filter: .all)
    }
  }
}
