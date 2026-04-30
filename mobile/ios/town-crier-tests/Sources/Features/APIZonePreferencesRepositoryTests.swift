import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("APIZonePreferencesRepository")
struct APIZonePreferencesRepositoryTests {

  // MARK: - Helpers

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APIZonePreferencesRepository, SpyAuthenticationService, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIZonePreferencesRepository(apiClient: apiClient)
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

  // MARK: - fetchPreferences

  @Test("fetchPreferences sends GET /v1/me/watch-zones/{zoneId}/preferences")
  func fetchPreferences_sendsCorrectRequest() async throws {
    let json = """
      {
        "zoneId": "zone-001",
        "newApplicationPush": true,
        "newApplicationEmail": false,
        "decisionPush": true,
        "decisionEmail": false
      }
      """
    let (sut, _, transport) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.fetchPreferences(zoneId: "zone-001")

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    #expect(request.url?.path().contains("/v1/me/watch-zones/zone-001/preferences") == true)

    #expect(result.zoneId == "zone-001")
    #expect(result.newApplicationPush == true)
    #expect(result.newApplicationEmail == false)
    #expect(result.decisionPush == true)
    #expect(result.decisionEmail == false)
  }

  @Test("fetchPreferences maps all-true response correctly")
  func fetchPreferences_allTrue_mapsCorrectly() async throws {
    let json = """
      {
        "zoneId": "zone-002",
        "newApplicationPush": true,
        "newApplicationEmail": true,
        "decisionPush": true,
        "decisionEmail": true
      }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.fetchPreferences(zoneId: "zone-002")

    #expect(result.newApplicationPush == true)
    #expect(result.newApplicationEmail == true)
    #expect(result.decisionPush == true)
    #expect(result.decisionEmail == true)
  }

  @Test("fetchPreferences with network error throws networkUnavailable")
  func fetchPreferences_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIZonePreferencesRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.fetchPreferences(zoneId: "zone-001")
    }
  }

  @Test("fetchPreferences with server error throws serverError not networkUnavailable")
  func fetchPreferences_serverError_throwsServerError() async {
    let (sut, _, _) = makeSUT(responses: [
      (Data("Internal Server Error".utf8), httpResponse(statusCode: 500))
    ])

    await #expect(
      throws: DomainError.serverError(statusCode: 500, message: "Internal Server Error")
    ) {
      _ = try await sut.fetchPreferences(zoneId: "zone-001")
    }
  }

  // MARK: - updatePreferences

  @Test("updatePreferences sends PUT /v1/me/watch-zones/{zoneId}/preferences with correct body")
  func updatePreferences_sendsCorrectRequest() async throws {
    let (sut, _, transport) = makeSUT(responses: [
      (Data("{}".utf8), httpResponse(statusCode: 200))
    ])
    let prefs = ZoneNotificationPreferences(
      zoneId: "zone-001",
      newApplicationPush: true,
      newApplicationEmail: false,
      decisionPush: true,
      decisionEmail: false
    )

    try await sut.updatePreferences(prefs)

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "PUT")
    #expect(request.url?.path().contains("/v1/me/watch-zones/zone-001/preferences") == true)

    let body = try #require(request.httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: Any])
    #expect(json["zoneId"] as? String == "zone-001")
    #expect(json["newApplicationPush"] as? Bool == true)
    #expect(json["newApplicationEmail"] as? Bool == false)
    #expect(json["decisionPush"] as? Bool == true)
    #expect(json["decisionEmail"] as? Bool == false)
  }

  @Test("updatePreferences with network error throws networkUnavailable")
  func updatePreferences_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIZonePreferencesRepository(apiClient: apiClient)
    let prefs = ZoneNotificationPreferences(zoneId: "zone-001")

    await #expect(throws: DomainError.networkUnavailable) {
      try await sut.updatePreferences(prefs)
    }
  }
}
