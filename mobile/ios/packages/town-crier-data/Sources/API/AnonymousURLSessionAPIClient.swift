import Foundation
import TownCrierDomain

/// Foundation HTTP client for the anonymous (pre-signup) browse surface
/// (GH#868 Phase 3): the same decode / error-mapping / paged-request shape as
/// ``URLSessionAPIClient``, but with no `AuthenticationService`, no
/// `Authorization` header, and no 401 token-refresh recovery — there is no
/// session to refresh. A missing/failed request surfaces a domain error
/// directly. Deliberately a parallel type rather than making
/// `URLSessionAPIClient`'s session requirement optional, so that client's
/// session guard — a safety property every authenticated repository relies on
/// — is never weakened.
public final class AnonymousURLSessionAPIClient: Sendable {
  private let baseURL: URL
  private let transport: HTTPTransport
  private let decoder: JSONDecoder
  private let encoder: JSONEncoder

  public init(
    baseURL: URL,
    transport: HTTPTransport = URLSession.shared
  ) {
    self.baseURL = baseURL
    self.transport = transport
    self.decoder = JSONDecoder()
    self.encoder = JSONEncoder()
  }

  public func request<T: Decodable & Sendable>(_ endpoint: APIEndpoint) async throws -> T {
    try await performRequest(endpoint).value
  }

  /// Performs a request and returns the decoded body alongside the opaque
  /// `X-Next-Cursor` continuation token from the response headers — `nil`
  /// when the header is absent (i.e. the last page). Mirrors
  /// ``URLSessionAPIClient/requestPaged(_:)``.
  public func requestPaged<T: Decodable & Sendable>(
    _ endpoint: APIEndpoint
  ) async throws -> (value: T, nextCursor: String?) {
    let result: (value: T, response: HTTPURLResponse) = try await performRequest(endpoint)
    return (result.value, result.response.value(forHTTPHeaderField: "X-Next-Cursor"))
  }

  private func performRequest<T: Decodable>(
    _ endpoint: APIEndpoint
  ) async throws -> (value: T, response: HTTPURLResponse) {
    let urlRequest = try buildRequest(endpoint)

    let data: Data
    let response: URLResponse
    do {
      (data, response) = try await transport.data(for: urlRequest)
    } catch is URLError {
      throw DomainError.networkUnavailable
    }

    guard let httpResponse = response as? HTTPURLResponse else {
      throw APIError.serverError(statusCode: 0, message: "Invalid response")
    }

    try mapHTTPStatus(httpResponse.statusCode, data: data)

    if T.self == EmptyResponse.self {
      // swiftlint:disable:next force_cast
      return (EmptyResponse() as! T, httpResponse)
    }

    do {
      return (try decoder.decode(T.self, from: data), httpResponse)
    } catch {
      throw APIError.decodingFailed(error.localizedDescription)
    }
  }

  private func buildRequest(_ endpoint: APIEndpoint) throws -> URLRequest {
    guard
      var components = URLComponents(
        url: baseURL.appendingPathComponent(endpoint.path),
        resolvingAgainstBaseURL: false
      )
    else {
      throw APIError.serverError(statusCode: 0, message: "Invalid URL components")
    }
    if let queryItems = endpoint.queryItems, !queryItems.isEmpty {
      components.queryItems = queryItems
    }

    guard let url = components.url else {
      throw APIError.serverError(statusCode: 0, message: "Invalid URL")
    }

    var request = URLRequest(url: url)
    request.httpMethod = endpoint.method.rawValue
    request.setValue("application/json", forHTTPHeaderField: "Accept")

    if let body = endpoint.body {
      request.setValue("application/json", forHTTPHeaderField: "Content-Type")
      request.httpBody = try encoder.encode(body)
    }

    return request
  }

  private func mapHTTPStatus(_ statusCode: Int, data: Data) throws {
    switch statusCode {
    case 200...299:
      return
    case 401:
      throw APIError.unauthorized
    case 404:
      throw APIError.notFound
    default:
      if statusCode >= 400 {
        let message = String(data: data, encoding: .utf8)
        throw APIError.serverError(statusCode: statusCode, message: message)
      }
    }
  }
}
