import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("APISavedApplicationRepository")
struct APISavedApplicationRepositoryTests {

  // MARK: - Helpers

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APISavedApplicationRepository, SpyAuthenticationService, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APISavedApplicationRepository(apiClient: apiClient)
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

  // MARK: - save

  @Test("save sends PUT /v1/me/saved-applications/{uid} with full PlanningApplication body")
  func save_sendsCorrectRequest() async throws {
    let (sut, _, transport) = makeSUT(responses: [
      (Data(), httpResponse(statusCode: 204))
    ])
    let app = PlanningApplication(
      id: PlanningApplicationId("BK/2026/0042"),
      reference: ApplicationReference("2026/0042"),
      authority: LocalAuthority(code: "123", name: "Cambridge"),
      status: .undecided,
      receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
      description: "Erection of two-storey rear extension",
      address: "12 Mill Road, Cambridge, CB1 2AD"
    )

    try await sut.save(application: app)

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "PUT")
    let url = try #require(request.url)
    #expect(url.path().contains("/v1/me/saved-applications/BK/2026/0042"))
    let body = try #require(request.httpBody)
    let json = try #require(
      try JSONSerialization.jsonObject(with: body) as? [String: Any]
    )
    #expect(json["uid"] as? String == "BK/2026/0042")
    #expect(json["name"] as? String == "2026/0042")
    #expect(json["areaName"] as? String == "Cambridge")
    #expect(json["areaId"] as? Int == 123)
    #expect(json["address"] as? String == "12 Mill Road, Cambridge, CB1 2AD")
    #expect(json["description"] as? String == "Erection of two-storey rear extension")
    #expect(json["lastDifferent"] != nil)
  }

  @Test("save with network error throws networkUnavailable")
  func save_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APISavedApplicationRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      try await sut.save(application: .pendingReview)
    }
  }

  @Test("save with server error throws serverError not networkUnavailable")
  func save_serverError_throwsServerError() async {
    let (sut, _, _) = makeSUT(responses: [
      (Data("Bad Request".utf8), httpResponse(statusCode: 400))
    ])

    await #expect(throws: DomainError.serverError(statusCode: 400, message: "Bad Request")) {
      try await sut.save(application: .pendingReview)
    }
  }

  // MARK: - remove

  @Test("remove sends DELETE /v1/me/saved-applications/{uid}")
  func remove_sendsCorrectRequest() async throws {
    let (sut, _, transport) = makeSUT(responses: [
      (Data(), httpResponse(statusCode: 204))
    ])

    try await sut.remove(applicationUid: "BK/2026/0042")

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "DELETE")
    let url = try #require(request.url)
    #expect(url.path().contains("/v1/me/saved-applications/BK/2026/0042"))
  }

  @Test("remove with network error throws networkUnavailable")
  func remove_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APISavedApplicationRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      try await sut.remove(applicationUid: "UID-1")
    }
  }

  // MARK: - loadAll

  @Test("loadAll sends GET /v1/me/saved-applications")
  func loadAll_sendsCorrectRequest() async throws {
    let json = "[]"
    let (sut, _, transport) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    _ = try await sut.loadAll()

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    let url = try #require(request.url)
    #expect(url.path().contains("/v1/me/saved-applications"))
  }

  @Test("loadAll maps response to SavedApplication array")
  func loadAll_mapsResponse() async throws {
    let json = """
      [
        {
          "applicationUid": "BK/2026/0042",
          "savedAt": "2026-04-10T14:30:00Z",
          "application": {
            "name": "Rear extension at 12 Mill Road",
            "uid": "BK/2026/0042",
            "areaName": "Cambridge",
            "areaId": 123,
            "address": "12 Mill Road, Cambridge, CB1 2AD",
            "postcode": "CB1 2AD",
            "description": "Erection of two-storey rear extension",
            "appType": "Full Planning Application",
            "appState": "Undecided",
            "appSize": null,
            "startDate": "2026-04-01",
            "decidedDate": null,
            "consultedDate": null,
            "longitude": 0.1243,
            "latitude": 52.2043,
            "url": "https://planning.cambridge.gov.uk/2026/0042",
            "link": null,
            "lastDifferent": "2026-04-09"
          }
        }
      ]
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.loadAll()

    #expect(result.count == 1)
    #expect(result[0].applicationUid == "BK/2026/0042")
    #expect(result[0].application != nil)
    #expect(result[0].application?.address == "12 Mill Road, Cambridge, CB1 2AD")
  }

  @Test("loadAll returns empty array for empty response")
  func loadAll_emptyResponse() async throws {
    let json = "[]"
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.loadAll()

    #expect(result.isEmpty)
  }

  @Test("loadAll with network error throws networkUnavailable")
  func loadAll_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APISavedApplicationRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.loadAll()
    }
  }

  @Test("loadAll with 401 throws sessionExpired")
  func loadAll_401_throwsSessionExpired() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    authService.refreshSessionResult = .failure(DomainError.sessionExpired)
    let transport = StubHTTPTransport()
    transport.responses = [
      (Data("{}".utf8), httpResponse(statusCode: 401))
    ]
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APISavedApplicationRepository(apiClient: apiClient)

    await #expect(throws: DomainError.sessionExpired) {
      _ = try await sut.loadAll()
    }
  }
}
