import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// Wire tests for GH#879 Phase 2: the anonymous (no-session) detail read that
/// backs a signed-out share Universal Link and the anonymous detail screen's
/// stale-while-revalidate refresh.
@Suite("APIAnonymousApplicationDetailRepository")
struct APIAnonymousApplicationDetailRepositoryTests {

  // MARK: - Helpers

  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APIAnonymousApplicationDetailRepository, StubHTTPTransport) {
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)
    let sut = APIAnonymousApplicationDetailRepository(apiClient: apiClient)
    return (sut, transport)
  }

  // swiftlint:disable force_unwrapping
  private func httpResponse(statusCode: Int) -> HTTPURLResponse {
    HTTPURLResponse(url: baseURL, statusCode: statusCode, httpVersion: nil, headerFields: nil)!
  }
  // swiftlint:enable force_unwrapping

  private let successJSON = """
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
    """

  // MARK: - fetchApplication(bySlug:ref:)

  @Test("fetchApplication(bySlug:) sends GET /v1/applications/by-slug/{slug}/{ref} with no Authorization header")
  func fetchApplicationBySlug_sendsCorrectRequest() async throws {
    let (sut, transport) = makeSUT(
      responses: [(Data(successJSON.utf8), httpResponse(statusCode: 200))])

    let app = try await sut.fetchApplication(bySlug: "kingston", ref: "Kingston/25/02755/CLC")

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    #expect(request.value(forHTTPHeaderField: "Authorization") == nil)
    // The ref's slashes pass through into the path verbatim, exactly like the
    // authed by-slug read interpolates `ref`.
    #expect(
      request.url?.path().contains("/v1/applications/by-slug/kingston/Kingston/25/02755/CLC")
        == true)
    #expect(app.id == PlanningApplicationId(authority: "789", name: "Kingston/25/02755/CLC"))
    #expect(app.authority.slug == "kingston")
  }

  @Test("fetchApplication(bySlug:) with 404 throws applicationNotFound")
  func fetchApplicationBySlug_notFound_throwsApplicationNotFound() async throws {
    let (sut, _) = makeSUT(responses: [(Data("null".utf8), httpResponse(statusCode: 404))])

    await #expect(
      throws: DomainError.applicationNotFound(
        PlanningApplicationId(authority: "kingston", name: "Kingston/25/GONE"))
    ) {
      _ = try await sut.fetchApplication(bySlug: "kingston", ref: "Kingston/25/GONE")
    }
  }

  @Test("fetchApplication(bySlug:) with server error throws serverError not networkUnavailable")
  func fetchApplicationBySlug_serverError_throwsServerError() async throws {
    let (sut, _) = makeSUT(responses: [
      (Data("Internal Server Error".utf8), httpResponse(statusCode: 500))
    ])

    await #expect(
      throws: DomainError.serverError(statusCode: 500, message: "Internal Server Error")
    ) {
      _ = try await sut.fetchApplication(bySlug: "kingston", ref: "Kingston/25/02755/CLC")
    }
  }

  @Test("fetchApplication(bySlug:) with network error throws networkUnavailable")
  func fetchApplicationBySlug_networkError_throwsNetworkUnavailable() async throws {
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)
    let sut = APIAnonymousApplicationDetailRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.fetchApplication(bySlug: "kingston", ref: "Kingston/25/02755/CLC")
    }
  }
}
