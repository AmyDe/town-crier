import Foundation
import Testing
import TownCrierData
import TownCrierDomain

@Suite("APIVersionConfigService")
struct APIVersionConfigServiceTests {
  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  @Test("Sends GET request to /v1/version-config")
  func sendsGetToCorrectPath() async throws {
    let transport = StubHTTPTransport()
    let json = #"{"minimumVersion":"1.0.0"}"#
    transport.responses = [
      (Data(json.utf8), httpResponse(url: baseURL, statusCode: 200))
    ]

    let sut = APIVersionConfigService(baseURL: baseURL, transport: transport)

    _ = try await sut.fetchMinimumVersion()

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    #expect(request.url?.path().contains("v1/version-config") == true)
  }

  @Test("Parses valid version response into AppVersion")
  func parsesValidResponse() async throws {
    let transport = StubHTTPTransport()
    let json = #"{"minimumVersion":"2.1.3"}"#
    transport.responses = [
      (Data(json.utf8), httpResponse(url: baseURL, statusCode: 200))
    ]

    let sut = APIVersionConfigService(baseURL: baseURL, transport: transport)

    let result = try await sut.fetchMinimumVersion()

    #expect(result == AppVersion(major: 2, minor: 1, patch: 3))
  }

  @Test("Network error throws networkUnavailable")
  func networkErrorThrowsNetworkUnavailable() async {
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)

    let sut = APIVersionConfigService(baseURL: baseURL, transport: transport)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.fetchMinimumVersion()
    }
  }

  @Test("Non-2xx status throws unexpected error")
  func nonSuccessStatusThrowsUnexpected() async {
    let transport = StubHTTPTransport()
    transport.responses = [
      (Data(), httpResponse(url: baseURL, statusCode: 500))
    ]

    let sut = APIVersionConfigService(baseURL: baseURL, transport: transport)

    await #expect(throws: DomainError.self) {
      _ = try await sut.fetchMinimumVersion()
    }
  }

  @Test("Invalid JSON throws unexpected error")
  func invalidJsonThrowsUnexpected() async {
    let transport = StubHTTPTransport()
    transport.responses = [
      (Data("not json".utf8), httpResponse(url: baseURL, statusCode: 200))
    ]

    let sut = APIVersionConfigService(baseURL: baseURL, transport: transport)

    await #expect(throws: DomainError.self) {
      _ = try await sut.fetchMinimumVersion()
    }
  }

  @Test("Invalid version string in response throws unexpected error")
  func invalidVersionStringThrowsUnexpected() async {
    let transport = StubHTTPTransport()
    let json = #"{"minimumVersion":"not-a-version"}"#
    transport.responses = [
      (Data(json.utf8), httpResponse(url: baseURL, statusCode: 200))
    ]

    let sut = APIVersionConfigService(baseURL: baseURL, transport: transport)

    await #expect(throws: DomainError.self) {
      _ = try await sut.fetchMinimumVersion()
    }
  }

  @Test("Request does not include Authorization header")
  func noAuthorizationHeader() async throws {
    let transport = StubHTTPTransport()
    let json = #"{"minimumVersion":"1.0.0"}"#
    transport.responses = [
      (Data(json.utf8), httpResponse(url: baseURL, statusCode: 200))
    ]

    let sut = APIVersionConfigService(baseURL: baseURL, transport: transport)

    _ = try await sut.fetchMinimumVersion()

    let request = transport.requests[0]
    #expect(request.value(forHTTPHeaderField: "Authorization") == nil)
  }
}
