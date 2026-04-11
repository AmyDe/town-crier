import Foundation
import TownCrierData

/// Stub transport that records requests and returns preconfigured responses.
final class StubHTTPTransport: HTTPTransport, @unchecked Sendable {
  var responses: [(Data, URLResponse)] = []
  private var callIndex = 0
  private(set) var requests: [URLRequest] = []
  var error: (any Error)?

  func data(for request: URLRequest) async throws -> (Data, URLResponse) {
    requests.append(request)
    if let error { throw error }
    guard callIndex < responses.count else {
      throw URLError(.badServerResponse)
    }
    let response = responses[callIndex]
    callIndex += 1
    return response
  }
}

func httpResponse(url: URL, statusCode: Int) -> HTTPURLResponse {
  // swiftlint:disable:next force_unwrapping
  HTTPURLResponse(url: url, statusCode: statusCode, httpVersion: nil, headerFields: nil)!
}
