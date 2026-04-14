import Foundation
import Testing
import TownCrierData
import TownCrierDomain

private struct TestResponse: Decodable, Equatable, Sendable {
  let id: String
  let name: String
}

@Suite("URLSessionAPIClient -- Token Refresh Error Handling")
struct URLSessionAPIClientTokenRefreshTests {
  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    session: AuthSession? = .valid,
    refreshResult: Result<AuthSession, Error> = .success(.valid),
    responses: [(Data, URLResponse)] = []
  ) -> (URLSessionAPIClient, StubHTTPTransport, SpyAuthenticationService) {
    let transport = StubHTTPTransport()
    transport.responses = responses
    let auth = SpyAuthenticationService()
    auth.currentSessionResult = session
    auth.refreshSessionResult = refreshResult
    let client = URLSessionAPIClient(
      baseURL: baseURL,
      authService: auth,
      transport: transport
    )
    return (client, transport, auth)
  }

  @Test("On 401, when token refresh fails due to network error, throws networkUnavailable")
  func refreshNetworkFailureThrowsNetworkUnavailable() async throws {
    let (sut, _, _) = makeSUT(
      refreshResult: .failure(URLError(.notConnectedToInternet)),
      responses: [(Data(), httpResponse(url: baseURL, statusCode: 401))]
    )

    await #expect(throws: DomainError.networkUnavailable) {
      let _: TestResponse = try await sut.request(.get("/applications"))
    }
  }

  @Test("On 401, when refresh succeeds but retry returns server error, propagates server error")
  func retryAfterRefreshServerErrorPropagates() async throws {
    let (sut, _, _) = makeSUT(
      responses: [
        (Data(), httpResponse(url: baseURL, statusCode: 401)),
        (Data(), httpResponse(url: baseURL, statusCode: 500)),
      ]
    )

    await #expect(throws: APIError.self) {
      let _: TestResponse = try await sut.request(.get("/applications"))
    }
  }

  @Test("On 401, when refresh succeeds but retry has network error, throws networkUnavailable")
  func retryAfterRefreshNetworkErrorThrowsNetworkUnavailable() async throws {
    // First call returns 401, retry hits empty responses -> URLError from StubHTTPTransport
    let (sut, _, _) = makeSUT(
      responses: [(Data(), httpResponse(url: baseURL, statusCode: 401))]
    )

    await #expect(throws: DomainError.networkUnavailable) {
      let _: TestResponse = try await sut.request(.get("/applications"))
    }
  }
}
