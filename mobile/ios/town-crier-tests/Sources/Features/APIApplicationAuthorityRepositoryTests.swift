import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("APIApplicationAuthorityRepository")
struct APIApplicationAuthorityRepositoryTests {

  // MARK: - Helpers

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APIApplicationAuthorityRepository, SpyAuthenticationService, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIApplicationAuthorityRepository(apiClient: apiClient)
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

  // MARK: - fetchAuthorities (GET /v1/me/application-authorities)

  @Test("fetchAuthorities sends GET /v1/me/application-authorities")
  func fetchAuthorities_sendsCorrectRequest() async throws {
    let json = """
      {
        "authorities": [
          { "id": 123, "name": "Bath and NE Somerset", "areaType": "Unitary" }
        ],
        "count": 1
      }
      """
    let (sut, _, transport) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    _ = try await sut.fetchAuthorities()

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    #expect(request.url?.path().contains("/v1/me/application-authorities") == true)
  }

  @Test("fetchAuthorities maps authorities with id, name, and areaType")
  func fetchAuthorities_mapsResponse() async throws {
    let json = """
      {
        "authorities": [
          { "id": 123, "name": "Bath and NE Somerset", "areaType": "Unitary" },
          { "id": 456, "name": "Cambridge", "areaType": "District" }
        ],
        "count": 2
      }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.fetchAuthorities()

    #expect(result.count == 2)
    #expect(result.authorities.count == 2)

    #expect(result.authorities[0].code == "123")
    #expect(result.authorities[0].name == "Bath and NE Somerset")
    #expect(result.authorities[0].areaType == "Unitary")

    #expect(result.authorities[1].code == "456")
    #expect(result.authorities[1].name == "Cambridge")
    #expect(result.authorities[1].areaType == "District")
  }

  @Test("fetchAuthorities maps empty authorities list")
  func fetchAuthorities_emptyList() async throws {
    let json = """
      {
        "authorities": [],
        "count": 0
      }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.fetchAuthorities()

    #expect(result.authorities.isEmpty)
    #expect(result.count == .zero)
  }

  @Test("fetchAuthorities with network error throws networkUnavailable")
  func fetchAuthorities_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIApplicationAuthorityRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.fetchAuthorities()
    }
  }

  @Test("fetchAuthorities with server error throws serverError not networkUnavailable")
  func fetchAuthorities_serverError_throwsServerError() async {
    let (sut, _, _) = makeSUT(responses: [
      (Data("Bad Request".utf8), httpResponse(statusCode: 400))
    ])

    await #expect(throws: DomainError.serverError(statusCode: 400, message: "Bad Request")) {
      _ = try await sut.fetchAuthorities()
    }
  }

  @Test("fetchAuthorities with 403 insufficient entitlement throws insufficientEntitlement")
  func fetchAuthorities_insufficientEntitlement() async {
    let json = """
      { "error": "insufficient_entitlement", "required": "statusChangeAlerts" }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 403))
    ])

    await #expect(throws: DomainError.insufficientEntitlement(required: "statusChangeAlerts")) {
      _ = try await sut.fetchAuthorities()
    }
  }
}
