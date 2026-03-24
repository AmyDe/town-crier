import Foundation

/// Abstraction over URLSession for testability.
/// URLSession conforms to this via extension.
public protocol HTTPTransport: Sendable {
    func data(for request: URLRequest) async throws -> (Data, URLResponse)
}

extension URLSession: HTTPTransport {}
