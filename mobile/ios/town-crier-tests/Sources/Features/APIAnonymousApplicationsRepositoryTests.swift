import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("APIAnonymousApplicationsRepository")
struct APIAnonymousApplicationsRepositoryTests {

  // MARK: - Helpers

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APIAnonymousApplicationsRepository, StubHTTPTransport) {
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)
    let sut = APIAnonymousApplicationsRepository(apiClient: apiClient)
    return (sut, transport)
  }

  // swiftlint:disable force_unwrapping
  private func httpResponse(statusCode: Int) -> HTTPURLResponse {
    HTTPURLResponse(url: baseURL, statusCode: statusCode, httpVersion: nil, headerFields: nil)!
  }
  // swiftlint:enable force_unwrapping

  // MARK: - fetchNearby

  @Test("fetchNearby sends GET /v1/applications/near-point with lat/lng/radius/limit")
  func fetchNearby_sendsCorrectRequest() async throws {
    let (sut, transport) = makeSUT(responses: [(Data("[]".utf8), httpResponse(statusCode: 200))])

    _ = try await sut.fetchNearby(
      latitude: 52.2053, longitude: 0.1218, radiusMetres: 2000, limit: 200)

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    let url = try #require(request.url)
    #expect(url.path.contains("/v1/applications/near-point"))
    #expect(request.value(forHTTPHeaderField: "Authorization") == nil)
    let components = try #require(URLComponents(url: url, resolvingAgainstBaseURL: false))
    let queryItems = try #require(components.queryItems)
    #expect(queryItems.contains(URLQueryItem(name: "lat", value: "52.2053")))
    #expect(queryItems.contains(URLQueryItem(name: "lng", value: "0.1218")))
    #expect(queryItems.contains(URLQueryItem(name: "radius", value: "2000.0")))
    #expect(queryItems.contains(URLQueryItem(name: "limit", value: "200")))
  }

  @Test("fetchNearby maps NearbyResult-shaped JSON to PlanningApplication domain models")
  func fetchNearby_mapsToDomainModels() async throws {
    // Exactly the wire shape emitted by GET /v1/applications/near-point
    // (api-go internal/applications/result.go NearbyResult) -- no
    // latestUnreadEvent/authoritySlug, both optional on the reused DTO.
    let json = """
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
              "url": "https://planning.cambridge.gov.uk/2026/0042",
              "link": null,
              "lastDifferent": "2026-01-15T00:00:00+00:00"
          }
      ]
      """
    let (sut, _) = makeSUT(responses: [(Data(json.utf8), httpResponse(statusCode: 200))])

    let result = try await sut.fetchNearby(
      latitude: 52.2053, longitude: 0.1218, radiusMetres: 2000, limit: 200)

    #expect(result.count == 1)
    let app = result[0]
    #expect(app.id == PlanningApplicationId(authority: "123", name: "2026/0042"))
    #expect(app.reference == ApplicationReference("2026/0042"))
    #expect(app.authority.name == "Cambridge")
    #expect(app.status == ApplicationStatus.undecided)
    #expect(app.address == "12 Mill Road, Cambridge, CB1 2AD")
    let expectedLocation = try Coordinate(latitude: 52.2043, longitude: 0.1243)
    #expect(app.location == expectedLocation)
  }

  @Test("fetchNearby maps the additive authoritySlug field onto the authority when present")
  func fetchNearby_mapsAuthoritySlug_whenPresent() async throws {
    // GH#879 Phase 1 (#880): near-point now emits authoritySlug per result,
    // reusing the same PlanningApplicationDTO/toDomain() mapping the by-id
    // and by-slug detail reads already exercise — no separate mapping
    // needed on the iOS side. This is the confirming test for that finding.
    let json = """
      [
          {
              "name": "Kingston/25/02755/CLC",
              "uid": "app-003",
              "areaName": "Kingston upon Thames",
              "areaId": 789,
              "authoritySlug": "kingston",
              "address": "1 Market Place, Kingston, KT1 1JS",
              "postcode": "KT1 1JS",
              "description": "Certificate of lawfulness",
              "appType": "Full",
              "appState": "Undecided",
              "appSize": null,
              "startDate": "2026-02-01",
              "decidedDate": null,
              "consultedDate": null,
              "longitude": null,
              "latitude": null,
              "url": null,
              "link": null,
              "lastDifferent": "2026-02-01T00:00:00+00:00"
          }
      ]
      """
    let (sut, _) = makeSUT(responses: [(Data(json.utf8), httpResponse(statusCode: 200))])

    let result = try await sut.fetchNearby(
      latitude: 52.2053, longitude: 0.1218, radiusMetres: 2000, limit: 200)

    #expect(result.count == 1)
    #expect(result[0].authority.slug == "kingston")
  }

  @Test("fetchNearby with network error throws networkUnavailable")
  func fetchNearby_networkError_throwsNetworkUnavailable() async throws {
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)
    let sut = APIAnonymousApplicationsRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.fetchNearby(
        latitude: 52.2053, longitude: 0.1218, radiusMetres: 2000, limit: 200)
    }
  }

  @Test("fetchNearby with server error throws serverError")
  func fetchNearby_serverError_throwsServerError() async throws {
    let (sut, _) = makeSUT(responses: [(Data("Bad Request".utf8), httpResponse(statusCode: 400))])

    await #expect(throws: DomainError.serverError(statusCode: 400, message: "Bad Request")) {
      _ = try await sut.fetchNearby(
        latitude: 52.2053, longitude: 0.1218, radiusMetres: 2000, limit: 200)
    }
  }

  // MARK: - sort (GH#912 Phase 3)

  @Test("fetchNearby sends sort=recent when explicitly requested")
  func fetchNearby_sortRecent_sendsSortQueryParam() async throws {
    let (sut, transport) = makeSUT(responses: [(Data("[]".utf8), httpResponse(statusCode: 200))])

    _ = try await sut.fetchNearby(
      latitude: 52.2053, longitude: 0.1218, radiusMetres: 2000, limit: 200, sort: .recent)

    let url = try #require(transport.requests[0].url)
    let components = try #require(URLComponents(url: url, resolvingAgainstBaseURL: false))
    let queryItems = try #require(components.queryItems)
    #expect(queryItems.contains(URLQueryItem(name: "sort", value: "recent")))
  }

  @Test("fetchNearby sends sort=distance when explicitly requested")
  func fetchNearby_sortDistance_sendsSortQueryParam() async throws {
    let (sut, transport) = makeSUT(responses: [(Data("[]".utf8), httpResponse(statusCode: 200))])

    _ = try await sut.fetchNearby(
      latitude: 52.2053, longitude: 0.1218, radiusMetres: 2000, limit: 200, sort: .distance)

    let url = try #require(transport.requests[0].url)
    let components = try #require(URLComponents(url: url, resolvingAgainstBaseURL: false))
    let queryItems = try #require(components.queryItems)
    #expect(queryItems.contains(URLQueryItem(name: "sort", value: "distance")))
  }

  @Test("fetchNearby without an explicit sort defaults to distance (map-path compatibility)")
  func fetchNearby_noExplicitSort_defaultsToDistance() async throws {
    let (sut, transport) = makeSUT(responses: [(Data("[]".utf8), httpResponse(statusCode: 200))])

    _ = try await sut.fetchNearby(
      latitude: 52.2053, longitude: 0.1218, radiusMetres: 2000, limit: 200)

    let url = try #require(transport.requests[0].url)
    let components = try #require(URLComponents(url: url, resolvingAgainstBaseURL: false))
    let queryItems = try #require(components.queryItems)
    #expect(queryItems.contains(URLQueryItem(name: "sort", value: "distance")))
  }
}
