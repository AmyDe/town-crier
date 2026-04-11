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
        "newApplications": true,
        "statusChanges": false,
        "decisionUpdates": true
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
    #expect(result.newApplications == true)
    #expect(result.statusChanges == false)
    #expect(result.decisionUpdates == true)
  }

  @Test("fetchPreferences maps all-true response correctly")
  func fetchPreferences_allTrue_mapsCorrectly() async throws {
    let json = """
      {
        "zoneId": "zone-002",
        "newApplications": true,
        "statusChanges": true,
        "decisionUpdates": true
      }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.fetchPreferences(zoneId: "zone-002")

    #expect(result.newApplications == true)
    #expect(result.statusChanges == true)
    #expect(result.decisionUpdates == true)
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

  // MARK: - updatePreferences

  @Test("updatePreferences sends PUT /v1/me/watch-zones/{zoneId}/preferences with correct body")
  func updatePreferences_sendsCorrectRequest() async throws {
    let (sut, _, transport) = makeSUT(responses: [
      (Data("{}".utf8), httpResponse(statusCode: 200))
    ])
    let prefs = ZoneNotificationPreferences(
      zoneId: "zone-001",
      newApplications: true,
      statusChanges: false,
      decisionUpdates: true
    )

    try await sut.updatePreferences(prefs)

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "PUT")
    #expect(request.url?.path().contains("/v1/me/watch-zones/zone-001/preferences") == true)

    let body = try #require(request.httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: Any])
    #expect(json["zoneId"] as? String == "zone-001")
    #expect(json["newApplications"] as? Bool == true)
    #expect(json["statusChanges"] as? Bool == false)
    #expect(json["decisionUpdates"] as? Bool == true)
  }

  @Test("updatePreferences with 403 insufficient_entitlement throws insufficientEntitlement")
  func updatePreferences_403_throwsInsufficientEntitlement() async {
    let errorJson = """
      {
        "error": "insufficient_entitlement",
        "required": "statusChangeAlerts"
      }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(errorJson.utf8), httpResponse(statusCode: 403))
    ])
    let prefs = ZoneNotificationPreferences(
      zoneId: "zone-001",
      newApplications: true,
      statusChanges: true,
      decisionUpdates: false
    )

    await #expect(throws: DomainError.insufficientEntitlement(required: "statusChangeAlerts")) {
      try await sut.updatePreferences(prefs)
    }
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
