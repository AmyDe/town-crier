import Foundation

/// Describes an API endpoint with method, path, optional body, and query parameters.
public struct APIEndpoint: Sendable {
    public let method: HTTPMethod
    public let path: String
    public let body: (any Encodable & Sendable)?
    public let queryItems: [URLQueryItem]?

    public init(method: HTTPMethod, path: String, body: (any Encodable & Sendable)? = nil, queryItems: [URLQueryItem]? = nil) {
        self.method = method
        self.path = path
        self.body = body
        self.queryItems = queryItems
    }

    public static func get(_ path: String, query: [URLQueryItem]? = nil) -> APIEndpoint {
        APIEndpoint(method: .get, path: path, queryItems: query)
    }

    public static func post(_ path: String, body: some Encodable & Sendable) -> APIEndpoint {
        APIEndpoint(method: .post, path: path, body: body)
    }

    public static func put(_ path: String, body: some Encodable & Sendable) -> APIEndpoint {
        APIEndpoint(method: .put, path: path, body: body)
    }

    public static func delete(_ path: String) -> APIEndpoint {
        APIEndpoint(method: .delete, path: path)
    }

    public static func patch(_ path: String, body: some Encodable & Sendable) -> APIEndpoint {
        APIEndpoint(method: .patch, path: path, body: body)
    }
}

public enum HTTPMethod: String, Sendable {
    case get = "GET"
    case post = "POST"
    case put = "PUT"
    case delete = "DELETE"
    case patch = "PATCH"
}
