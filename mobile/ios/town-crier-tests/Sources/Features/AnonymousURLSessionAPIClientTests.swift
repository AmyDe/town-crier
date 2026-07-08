import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

// MARK: - Test helpers

private struct TestResponse: Decodable, Equatable, Sendable {
  let id: String
  let name: String
}

@Suite("AnonymousURLSessionAPIClient")
struct AnonymousURLSessionAPIClientTests {
  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  // swiftlint:disable force_unwrapping
  private func httpResponse(statusCode: Int, headers: [String: String]? = nil) -> HTTPURLResponse {
    HTTPURLResponse(url: baseURL, statusCode: statusCode, httpVersion: nil, headerFields: headers)!
  }
  // swiftlint:enable force_unwrapping

  // MARK: - No auth

  @Test("GET request carries no Authorization header")
  func getRequest_carriesNoAuthorizationHeader() async throws {
    let transport = StubHTTPTransport()
    let json = #"{"id":"1","name":"Test"}"#
    transport.responses = [(Data(json.utf8), httpResponse(statusCode: 200))]
    let sut = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)

    let _: TestResponse = try await sut.request(.get("/v1/applications/near-point"))

    #expect(transport.requests.count == 1)
    #expect(transport.requests[0].value(forHTTPHeaderField: "Authorization") == nil)
  }

  // MARK: - Decoding

  @Test("Successful JSON response is decoded to expected type")
  func successfulJsonDecoding() async throws {
    let transport = StubHTTPTransport()
    let json = #"{"id":"42","name":"Extension"}"#
    transport.responses = [(Data(json.utf8), httpResponse(statusCode: 200))]
    let sut = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)

    let result: TestResponse = try await sut.request(.get("/v1/applications/near-point"))

    #expect(result == TestResponse(id: "42", name: "Extension"))
  }

  // MARK: - Paged requests

  @Test("requestPaged extracts the X-Next-Cursor header")
  func requestPaged_extractsNextCursorHeader() async throws {
    let transport = StubHTTPTransport()
    let json = #"[{"id":"1","name":"Test"}]"#
    transport.responses = [
      (Data(json.utf8), httpResponse(statusCode: 200, headers: ["X-Next-Cursor": "abc123"]))
    ]
    let sut = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)

    let result: (value: [TestResponse], nextCursor: String?) = try await sut.requestPaged(
      .get("/v1/applications/near-point"))

    #expect(result.value == [TestResponse(id: "1", name: "Test")])
    #expect(result.nextCursor == "abc123")
  }

  @Test("requestPaged returns nil cursor when the header is absent (last page)")
  func requestPaged_nilCursor_whenHeaderAbsent() async throws {
    let transport = StubHTTPTransport()
    let json = #"[]"#
    transport.responses = [(Data(json.utf8), httpResponse(statusCode: 200))]
    let sut = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)

    let result: (value: [TestResponse], nextCursor: String?) = try await sut.requestPaged(
      .get("/v1/applications/near-point"))

    #expect(result.nextCursor == nil)
  }

  // MARK: - Error mapping (no auth recovery)

  @Test("401 throws immediately with no refresh attempt")
  func unauthorized_throwsImmediately_noRefreshAttempt() async throws {
    let transport = StubHTTPTransport()
    transport.responses = [(Data(), httpResponse(statusCode: 401))]
    let sut = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)

    await #expect(throws: APIError.self) {
      let _: TestResponse = try await sut.request(.get("/v1/applications/near-point"))
    }
    // No retry: exactly the one request was ever sent.
    #expect(transport.requests.count == 1)
  }

  @Test("404 response throws APIError.notFound")
  func notFoundResponse() async throws {
    let transport = StubHTTPTransport()
    transport.responses = [(Data(), httpResponse(statusCode: 404))]
    let sut = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)

    await #expect(throws: APIError.self) {
      let _: TestResponse = try await sut.request(.get("/v1/applications/near-point"))
    }
  }

  @Test("500 response throws APIError.serverError")
  func serverErrorResponse() async throws {
    let transport = StubHTTPTransport()
    transport.responses = [(Data("boom".utf8), httpResponse(statusCode: 500))]
    let sut = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)

    await #expect(throws: APIError.self) {
      let _: TestResponse = try await sut.request(.get("/v1/applications/near-point"))
    }
  }

  @Test("Network error maps to DomainError.networkUnavailable")
  func networkErrorMapping() async throws {
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let sut = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)

    await #expect(throws: DomainError.networkUnavailable) {
      let _: TestResponse = try await sut.request(.get("/v1/applications/near-point"))
    }
  }

  // MARK: - Query parameters

  @Test("Request includes query parameters in URL")
  func queryParameters() async throws {
    let transport = StubHTTPTransport()
    let json = #"{"id":"1","name":"Test"}"#
    transport.responses = [(Data(json.utf8), httpResponse(statusCode: 200))]
    let sut = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)

    let endpoint = APIEndpoint.get(
      "/v1/applications/near-point",
      query: [
        URLQueryItem(name: "lat", value: "52.2053"),
        URLQueryItem(name: "lng", value: "0.1218"),
      ])

    let _: TestResponse = try await sut.request(endpoint)

    let url = try #require(transport.requests[0].url)
    let components = try #require(URLComponents(url: url, resolvingAgainstBaseURL: false))
    let queryItems = try #require(components.queryItems)
    #expect(queryItems.contains(URLQueryItem(name: "lat", value: "52.2053")))
    #expect(queryItems.contains(URLQueryItem(name: "lng", value: "0.1218")))
  }
}
