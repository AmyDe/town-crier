import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("APINotificationRepository")
struct APINotificationRepositoryTests {

  // MARK: - Helpers

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APINotificationRepository, SpyAuthenticationService, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APINotificationRepository(apiClient: apiClient)
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

  @Test("fetch sends GET /v1/me/notifications with page and pageSize params")
  func fetch_sendsCorrectRequest() async throws {
    let json = """
      { "notifications": [], "total": 0, "page": 1 }
      """
    let (sut, _, transport) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    _ = try await sut.fetch(page: 2, pageSize: 15)

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    let url = try #require(request.url)
    #expect(url.path().contains("/v1/me/notifications"))
    let query = try #require(url.query())
    #expect(query.contains("page=2"))
    #expect(query.contains("pageSize=15"))
  }

  // MARK: - Response mapping

  @Test("fetch maps response to NotificationPage with domain models")
  func fetch_mapsResponseToDomain() async throws {
    let json = """
      {
        "notifications": [
          {
            "applicationName": "Rear extension at 12 Mill Road",
            "applicationAddress": "12 Mill Road, Cambridge, CB1 2AD",
            "applicationDescription": "Erection of two-storey rear extension",
            "applicationType": "Full Planning Application",
            "authorityId": 123,
            "createdAt": "2026-04-10T14:30:00Z",
            "eventType": "DecisionUpdate",
            "decision": "Permitted",
            "sources": "Zone, Saved"
          }
        ],
        "total": 42,
        "page": 1
      }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.fetch(page: 1, pageSize: 20)

    #expect(result.notifications.count == 1)
    #expect(result.total == 42)
    #expect(result.page == 1)
    let item = result.notifications[0]
    #expect(item.applicationName == "Rear extension at 12 Mill Road")
    #expect(item.applicationAddress == "12 Mill Road, Cambridge, CB1 2AD")
    #expect(item.applicationDescription == "Erection of two-storey rear extension")
    #expect(item.applicationType == "Full Planning Application")
    #expect(item.authorityId == 123)
    #expect(item.eventType == "DecisionUpdate")
    #expect(item.decision == "Permitted")
    #expect(item.sources == "Zone, Saved")
  }

  @Test("fetch maps NewApplication event with null decision")
  func fetch_newApplicationEvent_nullDecision() async throws {
    let json = """
      {
        "notifications": [
          {
            "applicationName": "New app",
            "applicationAddress": "Address",
            "applicationDescription": "Description",
            "applicationType": "Full",
            "authorityId": 1,
            "createdAt": "2026-04-10T14:30:00Z",
            "eventType": "NewApplication",
            "decision": null,
            "sources": "Zone"
          }
        ],
        "total": 1,
        "page": 1
      }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.fetch(page: 1, pageSize: 20)

    let item = result.notifications[0]
    #expect(item.eventType == "NewApplication")
    #expect(item.decision == nil)
    #expect(item.sources == "Zone")
  }

  @Test("fetch tolerates missing eventType, decision, sources for older API responses")
  func fetch_missingNewFields_defaultsApplied() async throws {
    let json = """
      {
        "notifications": [
          {
            "applicationName": "Legacy",
            "applicationAddress": "Address",
            "applicationDescription": "Description",
            "applicationType": "Full",
            "authorityId": 1,
            "createdAt": "2026-04-10T14:30:00Z"
          }
        ],
        "total": 1,
        "page": 1
      }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.fetch(page: 1, pageSize: 20)

    let item = result.notifications[0]
    #expect(item.eventType == "NewApplication")
    #expect(item.decision == nil)
    #expect(item.sources.isEmpty)
  }

  @Test("fetch maps empty response correctly")
  func fetch_emptyResponse_returnsEmptyResult() async throws {
    let json = """
      { "notifications": [], "total": 0, "page": 1 }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.fetch(page: 1, pageSize: 20)

    #expect(result.notifications.isEmpty)
    #expect(result.total == 0)
    #expect(result.page == 1)
  }

  @Test("fetch maps multiple notifications correctly")
  func fetch_multipleNotifications_mapsAll() async throws {
    let json = """
      {
        "notifications": [
          {
            "applicationName": "App A",
            "applicationAddress": "Address A",
            "applicationDescription": "Description A",
            "applicationType": "Full",
            "authorityId": 123,
            "createdAt": "2026-04-10T14:30:00Z"
          },
          {
            "applicationName": "App B",
            "applicationAddress": "Address B",
            "applicationDescription": "Description B",
            "applicationType": "Householder",
            "authorityId": 456,
            "createdAt": "2026-04-09T10:00:00Z"
          }
        ],
        "total": 10,
        "page": 1
      }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.fetch(page: 1, pageSize: 20)

    #expect(result.notifications.count == 2)
    #expect(result.notifications[0].applicationName == "App A")
    #expect(result.notifications[1].applicationName == "App B")
    #expect(result.total == 10)
  }

  // MARK: - Error handling

  @Test("fetch with network error throws networkUnavailable")
  func fetch_networkError_throwsNetworkUnavailable() async throws {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APINotificationRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.fetch(page: 1, pageSize: 20)
    }
  }

  @Test("fetch with server error throws serverError not networkUnavailable")
  func fetch_serverError_throwsServerError() async throws {
    let (sut, _, _) = makeSUT(responses: [
      (Data("Bad Request".utf8), httpResponse(statusCode: 400))
    ])

    await #expect(throws: DomainError.serverError(statusCode: 400, message: "Bad Request")) {
      _ = try await sut.fetch(page: 1, pageSize: 20)
    }
  }

  @Test("fetch with 401 throws sessionExpired")
  func fetch_401_throwsSessionExpired() async throws {
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
    let sut = APINotificationRepository(apiClient: apiClient)

    await #expect(throws: DomainError.sessionExpired) {
      _ = try await sut.fetch(page: 1, pageSize: 20)
    }
  }
}
