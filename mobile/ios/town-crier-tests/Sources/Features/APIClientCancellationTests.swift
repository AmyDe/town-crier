import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

private struct TestResponse: Decodable, Equatable, Sendable {
  let id: String
}

@Suite("API clients -- cancellation passes through unchanged")
struct APIClientCancellationTests {
  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeAuthenticated(
    transportError: (any Error)? = nil,
    refreshResult: Result<AuthSession, Error> = .success(.valid),
    responses: [(Data, URLResponse)] = []
  ) -> URLSessionAPIClient {
    let transport = StubHTTPTransport()
    transport.error = transportError
    transport.responses = responses
    let auth = SpyAuthenticationService()
    auth.currentSessionResult = .valid
    auth.refreshSessionResult = refreshResult
    return URLSessionAPIClient(baseURL: baseURL, authService: auth, transport: transport)
  }

  private func makeAnonymous(transportError: any Error) -> AnonymousURLSessionAPIClient {
    let transport = StubHTTPTransport()
    transport.error = transportError
    return AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)
  }

  @Test("Authenticated request rethrows CancellationError")
  func authenticatedRequest_cancellationError() async {
    let sut = makeAuthenticated(transportError: CancellationError())
    await #expect(throws: CancellationError.self) {
      let _: TestResponse = try await sut.request(.get("/applications"))
    }
  }

  @Test("Authenticated request rethrows URLError.cancelled")
  func authenticatedRequest_urlErrorCancelled() async {
    let sut = makeAuthenticated(transportError: URLError(.cancelled))
    await #expect {
      let _: TestResponse = try await sut.request(.get("/applications"))
    } throws: { error in
      (error as? URLError)?.code == .cancelled
    }
  }

  @Test("Authenticated requestData rethrows CancellationError")
  func authenticatedRequestData_cancellationError() async {
    let sut = makeAuthenticated(transportError: CancellationError())
    await #expect(throws: CancellationError.self) {
      _ = try await sut.requestData(.get("/me/data"))
    }
  }

  @Test("Authenticated requestData rethrows URLError.cancelled")
  func authenticatedRequestData_urlErrorCancelled() async {
    let sut = makeAuthenticated(transportError: URLError(.cancelled))
    await #expect {
      _ = try await sut.requestData(.get("/me/data"))
    } throws: { error in
      (error as? URLError)?.code == .cancelled
    }
  }

  @Test("Cancelled token refresh after 401 is not mapped to sessionExpired")
  func refreshCancelled_notSessionExpired() async {
    let sut = makeAuthenticated(
      refreshResult: .failure(CancellationError()),
      responses: [(Data(), httpResponse(url: baseURL, statusCode: 401))]
    )
    await #expect(throws: CancellationError.self) {
      let _: TestResponse = try await sut.request(.get("/applications"))
    }
  }

  @Test("URLError.cancelled during token refresh is not mapped to networkUnavailable")
  func refreshUrlErrorCancelled_notNetworkUnavailable() async {
    let sut = makeAuthenticated(
      refreshResult: .failure(URLError(.cancelled)),
      responses: [(Data(), httpResponse(url: baseURL, statusCode: 401))]
    )
    await #expect {
      let _: TestResponse = try await sut.request(.get("/applications"))
    } throws: { error in
      (error as? URLError)?.code == .cancelled
    }
  }

  @Test("Anonymous request rethrows CancellationError")
  func anonymousRequest_cancellationError() async {
    let sut = makeAnonymous(transportError: CancellationError())
    await #expect(throws: CancellationError.self) {
      let _: TestResponse = try await sut.request(.get("/v1/applications/near-point"))
    }
  }

  @Test("Anonymous request rethrows URLError.cancelled")
  func anonymousRequest_urlErrorCancelled() async {
    let sut = makeAnonymous(transportError: URLError(.cancelled))
    await #expect {
      let _: TestResponse = try await sut.request(.get("/v1/applications/near-point"))
    } throws: { error in
      (error as? URLError)?.code == .cancelled
    }
  }

  @Test("Other URLError codes still map to networkUnavailable")
  func anonymousRequest_otherUrlError_stillMapped() async {
    let sut = makeAnonymous(transportError: URLError(.notConnectedToInternet))
    await #expect(throws: DomainError.networkUnavailable) {
      let _: TestResponse = try await sut.request(.get("/v1/applications/near-point"))
    }
  }

  @Test("toThrowableDomainError keeps cancellation and maps other errors")
  func toThrowableDomainError() {
    #expect(URLError(.cancelled).toThrowableDomainError() is URLError)
    #expect(CancellationError().toThrowableDomainError() is CancellationError)
    #expect(URLError(.timedOut).toThrowableDomainError() as? DomainError == .networkUnavailable)
  }

  @Test("isCancellation is true for both cancellation forms only")
  func isCancellation() {
    #expect(CancellationError().isCancellation)
    #expect(URLError(.cancelled).isCancellation)
    #expect(!URLError(.timedOut).isCancellation)
    #expect(!DomainError.networkUnavailable.isCancellation)
  }
}
