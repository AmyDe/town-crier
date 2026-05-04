import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("APINotificationStateRepository")
struct APINotificationStateRepositoryTests {

  // MARK: - Helpers

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APINotificationStateRepository, SpyAuthenticationService, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APINotificationStateRepository(apiClient: apiClient)
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

  // MARK: - fetchState request format

  @Test("fetchState sends GET /v1/me/notification-state")
  func fetchState_sendsCorrectRequest() async throws {
    let json = """
      { "lastReadAt": "2026-04-10T14:30:00Z", "version": 1, "totalUnreadCount": 0 }
      """
    let (sut, _, transport) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    _ = try await sut.fetchState()

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    let url = try #require(request.url)
    #expect(url.path().contains("/v1/me/notification-state"))
    // Must NOT match advance/mark-all-read sub-paths.
    #expect(!url.path().contains("advance"))
    #expect(!url.path().contains("mark-all-read"))
  }

  // MARK: - fetchState response mapping

  @Test("fetchState maps lastReadAt, version, and totalUnreadCount to domain")
  func fetchState_mapsResponseToDomain() async throws {
    let json = """
      {
        "lastReadAt": "2026-04-10T14:30:00Z",
        "version": 7,
        "totalUnreadCount": 12
      }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let state = try await sut.fetchState()

    #expect(state.version == 7)
    #expect(state.totalUnreadCount == 12)
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime]
    let expected = try #require(formatter.date(from: "2026-04-10T14:30:00Z"))
    #expect(state.lastReadAt == expected)
  }

  @Test("fetchState maps zero unread count")
  func fetchState_zeroUnread_mapsCorrectly() async throws {
    let json = """
      { "lastReadAt": "2026-04-10T14:30:00Z", "version": 1, "totalUnreadCount": 0 }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let state = try await sut.fetchState()

    #expect(state.totalUnreadCount == 0)
    #expect(!state.hasUnread)
  }

  // MARK: - fetchState error handling

  @Test("fetchState with network error throws networkUnavailable")
  func fetchState_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APINotificationStateRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.fetchState()
    }
  }

  @Test("fetchState with 401 throws sessionExpired")
  func fetchState_401_throwsSessionExpired() async {
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
    let sut = APINotificationStateRepository(apiClient: apiClient)

    await #expect(throws: DomainError.sessionExpired) {
      _ = try await sut.fetchState()
    }
  }

  @Test("fetchState with server error throws serverError")
  func fetchState_serverError_throwsServerError() async {
    let (sut, _, _) = makeSUT(responses: [
      (Data("Bad Request".utf8), httpResponse(statusCode: 400))
    ])

    await #expect(throws: DomainError.serverError(statusCode: 400, message: "Bad Request")) {
      _ = try await sut.fetchState()
    }
  }

  // MARK: - markAllRead

  @Test("markAllRead sends POST /v1/me/notification-state/mark-all-read with no body")
  func markAllRead_sendsCorrectRequest() async throws {
    let (sut, _, transport) = makeSUT(responses: [
      (Data(), httpResponse(statusCode: 204))
    ])

    try await sut.markAllRead()

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "POST")
    let url = try #require(request.url)
    #expect(url.path().contains("/v1/me/notification-state/mark-all-read"))
    #expect(request.httpBody == nil)
  }

  @Test("markAllRead 204 returns without throwing")
  func markAllRead_204_succeeds() async throws {
    let (sut, _, _) = makeSUT(responses: [
      (Data(), httpResponse(statusCode: 204))
    ])

    try await sut.markAllRead()
  }

  @Test("markAllRead with network error throws networkUnavailable")
  func markAllRead_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APINotificationStateRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      try await sut.markAllRead()
    }
  }

  // MARK: - advance

  @Test("advance sends POST /v1/me/notification-state/advance with asOf body")
  func advance_sendsCorrectRequest() async throws {
    let (sut, _, transport) = makeSUT(responses: [
      (Data(), httpResponse(statusCode: 204))
    ])
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime]
    let asOf = try #require(formatter.date(from: "2026-05-01T12:00:00Z"))

    try await sut.advance(asOf: asOf)

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "POST")
    let url = try #require(request.url)
    #expect(url.path().contains("/v1/me/notification-state/advance"))

    let body = try #require(request.httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: Any])
    let asOfString = try #require(json["asOf"] as? String)
    // Server parses ISO-8601; the exact spelling does not matter as long as it
    // round-trips to the same instant.
    let parsed = try #require(formatter.date(from: asOfString))
    #expect(parsed == asOf)
  }

  @Test("advance sets Content-Type to application/json")
  func advance_setsContentTypeJson() async throws {
    let (sut, _, transport) = makeSUT(responses: [
      (Data(), httpResponse(statusCode: 204))
    ])

    try await sut.advance(asOf: Date(timeIntervalSince1970: 1_712_000_000))

    let request = transport.requests[0]
    #expect(request.value(forHTTPHeaderField: "Content-Type") == "application/json")
  }

  @Test("advance 204 returns without throwing")
  func advance_204_succeeds() async throws {
    let (sut, _, _) = makeSUT(responses: [
      (Data(), httpResponse(statusCode: 204))
    ])

    try await sut.advance(asOf: Date(timeIntervalSince1970: 1_712_000_000))
  }

  @Test("advance with network error throws networkUnavailable")
  func advance_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APINotificationStateRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      try await sut.advance(asOf: Date(timeIntervalSince1970: 1_712_000_000))
    }
  }
}
