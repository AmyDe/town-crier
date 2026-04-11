import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("APIUserProfileRepository")
struct APIUserProfileRepositoryTests {

  // MARK: - Helpers

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APIUserProfileRepository, SpyAuthenticationService, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIUserProfileRepository(apiClient: apiClient)
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

  // MARK: - create (POST /v1/me)

  @Test("create sends POST /v1/me with no body")
  func create_sendsCorrectRequest() async throws {
    let json = """
      {
        "userId": "auth0|user-001",
        "pushEnabled": true,
        "tier": "Free"
      }
      """
    let (sut, _, transport) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let profile = try await sut.create()

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "POST")
    #expect(request.url?.path().contains("/v1/me") == true)
    #expect(request.httpBody == nil)
    #expect(profile.userId == "auth0|user-001")
    #expect(profile.tier == .free)
    #expect(profile.pushEnabled == true)
  }

  @Test("create with network error throws networkUnavailable")
  func create_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIUserProfileRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.create()
    }
  }

  // MARK: - fetch (GET /v1/me)

  @Test("fetch sends GET /v1/me and maps full response")
  func fetch_mapsFullResponse() async throws {
    let json = """
      {
        "userId": "auth0|user-001",
        "pushEnabled": false,
        "digestDay": "Wednesday",
        "emailDigestEnabled": true,
        "tier": "Personal"
      }
      """
    let (sut, _, transport) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let profile = try await sut.fetch()

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    #expect(request.url?.path().contains("/v1/me") == true)

    let result = try #require(profile)
    #expect(result.userId == "auth0|user-001")
    #expect(result.tier == .personal)
    #expect(result.pushEnabled == false)
    #expect(result.digestDay == .wednesday)
    #expect(result.emailDigestEnabled == true)
  }

  @Test("fetch returns nil on 404")
  func fetch_notFound_returnsNil() async throws {
    let (sut, _, _) = makeSUT(responses: [
      (Data("null".utf8), httpResponse(statusCode: 404))
    ])

    let profile = try await sut.fetch()

    #expect(profile == nil)
  }

  @Test("fetch with network error throws networkUnavailable")
  func fetch_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIUserProfileRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.fetch()
    }
  }

  // MARK: - update (PATCH /v1/me)

  @Test("update sends PATCH /v1/me with correct body")
  func update_sendsCorrectRequest() async throws {
    let json = """
      {
        "userId": "auth0|user-001",
        "pushEnabled": false,
        "digestDay": "Friday",
        "emailDigestEnabled": false,
        "tier": "Personal"
      }
      """
    let (sut, _, transport) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let updated = try await sut.update(
      pushEnabled: false,
      digestDay: .friday,
      emailDigestEnabled: false
    )

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "PATCH")
    #expect(request.url?.path().contains("/v1/me") == true)

    let body = try #require(request.httpBody)
    let bodyJSON = try #require(
      try JSONSerialization.jsonObject(with: body) as? [String: Any])
    #expect(bodyJSON["pushEnabled"] as? Bool == false)
    #expect(bodyJSON["digestDay"] as? String == "Friday")
    #expect(bodyJSON["emailDigestEnabled"] as? Bool == false)

    #expect(updated.pushEnabled == false)
    #expect(updated.digestDay == .friday)
    #expect(updated.tier == .personal)
  }

  @Test("update with network error throws networkUnavailable")
  func update_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIUserProfileRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.update(
        pushEnabled: true,
        digestDay: .monday,
        emailDigestEnabled: true
      )
    }
  }

  // MARK: - delete (DELETE /v1/me)

  @Test("delete sends DELETE /v1/me")
  func delete_sendsCorrectRequest() async throws {
    let (sut, _, transport) = makeSUT(responses: [
      (Data(), httpResponse(statusCode: 204))
    ])

    try await sut.delete()

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "DELETE")
    #expect(request.url?.path().contains("/v1/me") == true)
  }

  @Test("delete with network error throws networkUnavailable")
  func delete_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIUserProfileRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      try await sut.delete()
    }
  }

  @Test("delete with 404 throws networkUnavailable mapped from not found")
  func delete_notFound_throws() async {
    let (sut, _, _) = makeSUT(responses: [
      (Data("null".utf8), httpResponse(statusCode: 404))
    ])

    await #expect(throws: DomainError.self) {
      try await sut.delete()
    }
  }
}
