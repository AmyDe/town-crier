import Foundation

/// HTTP-level errors thrown by the API client.
/// Repositories should catch these and map to appropriate DomainError cases.
public enum APIError: Error, Sendable {
    case unauthorized
    case notFound
    case serverError(statusCode: Int, message: String?)
    case decodingFailed(String)
}
