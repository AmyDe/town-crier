import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("APISearchRepository")
struct APISearchRepositoryTests {

  // MARK: - Helpers

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APISearchRepository, SpyAuthenticationService, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APISearchRepository(apiClient: apiClient)
    return (sut, authService, transport)
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

  // MARK: - Request format

  @Test("search sends GET /v1/search with q, authorityId, and page params")
  func search_sendsCorrectRequest() async throws {
    let json = """
      { "applications": [], "total": 0, "page": 1 }
      """
    let (sut, _, transport) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    _ = try await sut.search(query: "extension", authorityId: 123, page: 2)

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    let url = try #require(request.url)
    #expect(url.path() == "/v1/search")
    let query = try #require(url.query())
    #expect(query.contains("q=extension"))
    #expect(!query.contains("query=extension"))
    #expect(query.contains("authorityId=123"))
    #expect(query.contains("page=2"))
  }

  // MARK: - Response mapping

  @Test("search maps response to SearchResult with domain models")
  func search_mapsResponseToDomain() async throws {
    let json = """
      {
        "applications": [
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
        ],
        "total": 42,
        "page": 1
      }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.search(query: "extension", authorityId: 123, page: 1)

    #expect(result.applications.count == 1)
    #expect(result.total == 42)
    #expect(result.page == 1)
    let app = result.applications[0]
    #expect(app.id == PlanningApplicationId("app-001"))
    #expect(app.reference == ApplicationReference("2026/0042"))
    #expect(app.status == .undecided)
    #expect(app.address == "12 Mill Road, Cambridge, CB1 2AD")
  }

  @Test("search maps empty response correctly")
  func search_emptyResponse_returnsEmptyResult() async throws {
    let json = """
      { "applications": [], "total": 0, "page": 1 }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.search(query: "nothing", authorityId: 123, page: 1)

    #expect(result.applications.isEmpty)
    #expect(result.total == 0)
    #expect(result.page == 1)
  }

  // MARK: - Error handling

  @Test("search with network error throws networkUnavailable")
  func search_networkError_throwsNetworkUnavailable() async throws {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APISearchRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.search(query: "test", authorityId: 123, page: 1)
    }
  }

  @Test("search with server error throws serverError not networkUnavailable")
  func search_serverError_throwsServerError() async throws {
    let (sut, _, _) = makeSUT(responses: [
      (Data("Bad Request".utf8), httpResponse(statusCode: 400))
    ])

    await #expect(throws: DomainError.serverError(statusCode: 400, message: "Bad Request")) {
      _ = try await sut.search(query: "test", authorityId: 123, page: 1)
    }
  }

  @Test("search with 403 insufficient_entitlement throws insufficientEntitlement")
  func search_403_throwsInsufficientEntitlement() async throws {
    let json = """
      { "error": "insufficient_entitlement", "required": "searchApplications" }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 403))
    ])

    await #expect(
      throws: DomainError.insufficientEntitlement(required: "searchApplications")
    ) {
      _ = try await sut.search(query: "test", authorityId: 123, page: 1)
    }
  }

  // MARK: - URL encoding

  @Test("search URL-encodes the query parameter")
  func search_urlEncodesQuery() async throws {
    let json = """
      { "applications": [], "total": 0, "page": 1 }
      """
    let (sut, _, transport) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    _ = try await sut.search(query: "rear extension & garage", authorityId: 123, page: 1)

    let url = try #require(transport.requests.first?.url)
    let query = try #require(url.query())
    // URLQueryItem handles encoding — the ampersand must be percent-encoded so
    // it does not split the query string into a separate parameter.
    #expect(!query.contains("authorityId=123&garage"))
    let components = try #require(URLComponents(url: url, resolvingAgainstBaseURL: false))
    let qItem = try #require(components.queryItems?.first { $0.name == "q" })
    #expect(qItem.value == "rear extension & garage")
  }
}
