import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// Covers the paged watch-zone applications fetch (GH#682 slice 1): it passes
/// `?sort=&cursor=&limit=`, decodes the bare `[PlanningApplicationDTO]` body, and
/// returns the page alongside the `X-Next-Cursor` continuation token (nil on the
/// last page). The map keeps using the param-less `fetchApplications(for:)`.
@Suite("APIPlanningApplicationRepository — paged fetch")
struct APIPlanningApplicationRepositoryPagedTests {
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

  private static let oneAppJSON = """
    [
        {
            "name": "2026/0042",
            "uid": "app-001",
            "areaName": "Cambridge",
            "areaId": 123,
            "address": "12 Mill Road, Cambridge, CB1 2AD",
            "postcode": "CB1 2AD",
            "description": "Erection of two-storey rear extension",
            "appType": "Full",
            "appState": "Undecided",
            "appSize": null,
            "startDate": "2026-01-15",
            "decidedDate": null,
            "consultedDate": null,
            "longitude": 0.1243,
            "latitude": 52.2043,
            "url": null,
            "link": null,
            "lastDifferent": "2026-01-15T00:00:00+00:00"
        }
    ]
    """

  @Test("first page sends ?sort= and ?limit= and omits ?cursor=")
  func fetchApplicationsPage_firstPage_sendsSortAndLimit() async throws {
    let (sut, transport) = makeSUT(responses: [
      (Data("[]".utf8), httpResponse(statusCode: 200, headers: [:]))
    ])

    _ = try await sut.fetchApplicationsPage(
      for: testZone, sort: .newest, cursor: nil, limit: 150)

    let url = try #require(transport.requests[0].url)
    #expect(url.path() == "/v1/me/watch-zones/zone-123/applications")
    let items = try queryItems(transport, at: 0)
    let hasCursor = items.contains { $0.name == "cursor" }
    #expect(items.contains(URLQueryItem(name: "sort", value: "newest")))
    #expect(items.contains(URLQueryItem(name: "limit", value: "150")))
    #expect(!hasCursor)
  }

  @Test("paging beyond the first page sends the cursor")
  func fetchApplicationsPage_withCursor_includesCursorParam() async throws {
    let (sut, transport) = makeSUT(responses: [
      (Data("[]".utf8), httpResponse(statusCode: 200, headers: [:]))
    ])

    _ = try await sut.fetchApplicationsPage(
      for: testZone, sort: .oldest, cursor: "abc123", limit: 150)

    let items = try queryItems(transport, at: 0)
    #expect(items.contains(URLQueryItem(name: "sort", value: "oldest")))
    #expect(items.contains(URLQueryItem(name: "cursor", value: "abc123")))
  }

  @Test("decodes the bare array body and returns the next cursor from the header")
  func fetchApplicationsPage_decodesBodyAndCursor() async throws {
    let (sut, _) = makeSUT(responses: [
      (Data(Self.oneAppJSON.utf8), httpResponse(statusCode: 200, headers: ["X-Next-Cursor": "next-1"]))
    ])

    let page = try await sut.fetchApplicationsPage(
      for: testZone, sort: .newest, cursor: nil, limit: 150)

    #expect(page.applications.count == 1)
    #expect(page.applications.first?.reference == ApplicationReference("2026/0042"))
    #expect(page.nextCursor == "next-1")
  }

  @Test("returns a nil cursor on the last page (no X-Next-Cursor header)")
  func fetchApplicationsPage_lastPage_nilCursor() async throws {
    let (sut, _) = makeSUT(responses: [
      (Data("[]".utf8), httpResponse(statusCode: 200, headers: [:]))
    ])

    let page = try await sut.fetchApplicationsPage(
      for: testZone, sort: .distance, cursor: nil, limit: 150)

    #expect(page.applications.isEmpty)
    #expect(page.nextCursor == nil)
  }

  @Test("maps a network error to DomainError.networkUnavailable")
  func fetchApplicationsPage_networkError_mapsToDomainError() async throws {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL, authService: authService, transport: transport)
    let sut = APIPlanningApplicationRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.fetchApplicationsPage(
        for: testZone, sort: .newest, cursor: nil, limit: 150)
    }
  }
}
