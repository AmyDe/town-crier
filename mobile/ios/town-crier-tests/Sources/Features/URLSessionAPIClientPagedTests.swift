import Foundation
import Testing
import TownCrierData
import TownCrierDomain

// MARK: - Test helpers

private struct PagedTestRow: Decodable, Equatable, Sendable {
  let id: String
}

// MARK: - Tests

/// Covers `requestPaged`, the variant of the API client used by the watch-zone
/// applications list to page the full set: it returns the decoded body plus the
/// opaque `X-Next-Cursor` continuation token from the response headers (GH#682).
@Suite("URLSessionAPIClient — paged requests (X-Next-Cursor)")
struct URLSessionAPIClientPagedTests {
  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    _ responses: [(Data, URLResponse)]
  ) -> (URLSessionAPIClient, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let client = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    return (client, transport)
  }

  private func response(statusCode: Int, headers: [String: String]) -> HTTPURLResponse {
    // swiftlint:disable:next force_unwrapping
    HTTPURLResponse(url: baseURL, statusCode: statusCode, httpVersion: nil, headerFields: headers)!
  }

  @Test("requestPaged surfaces the X-Next-Cursor response header")
  func requestPaged_surfacesNextCursor() async throws {
    let json = #"[{"id":"a"}]"#
    let (sut, _) = makeSUT([
      (Data(json.utf8), response(statusCode: 200, headers: ["X-Next-Cursor": "cursor-xyz"]))
    ])

    let page: (value: [PagedTestRow], nextCursor: String?) =
      try await sut.requestPaged(.get("/things"))

    #expect(page.value == [PagedTestRow(id: "a")])
    #expect(page.nextCursor == "cursor-xyz")
  }

  @Test("requestPaged returns a nil cursor when the header is absent (last page)")
  func requestPaged_nilCursorWhenHeaderAbsent() async throws {
    let json = #"[{"id":"a"}]"#
    let (sut, _) = makeSUT([
      (Data(json.utf8), response(statusCode: 200, headers: [:]))
    ])

    let page: (value: [PagedTestRow], nextCursor: String?) =
      try await sut.requestPaged(.get("/things"))

    #expect(page.nextCursor == nil)
  }
}
