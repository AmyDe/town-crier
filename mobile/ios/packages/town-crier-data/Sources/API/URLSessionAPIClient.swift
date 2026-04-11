import Foundation
import TownCrierDomain

/// Foundation HTTP client that injects Auth0 bearer tokens and handles
/// common response parsing, error mapping, and token refresh on 401.
public final class URLSessionAPIClient: Sendable {
    private let baseURL: URL
    private let authService: AuthenticationService
    private let transport: HTTPTransport
    private let decoder: JSONDecoder
    private let encoder: JSONEncoder

    public init(
        baseURL: URL,
        authService: AuthenticationService,
        transport: HTTPTransport = URLSession.shared
    ) {
        self.baseURL = baseURL
        self.authService = authService
        self.transport = transport
        self.decoder = JSONDecoder()
        self.encoder = JSONEncoder()
    }

    public func request<T: Decodable & Sendable>(_ endpoint: APIEndpoint) async throws -> T {
        guard let session = await authService.currentSession() else {
            throw DomainError.sessionExpired
        }

        do {
            return try await executeRequest(endpoint, accessToken: session.accessToken)
        } catch APIError.unauthorized {
            do {
                let refreshed = try await authService.refreshSession()
                return try await executeRequest(endpoint, accessToken: refreshed.accessToken)
            } catch {
                throw DomainError.sessionExpired
            }
        } catch is URLError {
            throw DomainError.networkUnavailable
        }
    }

    // MARK: - Private

    private func executeRequest<T: Decodable>(
        _ endpoint: APIEndpoint,
        accessToken: String
    ) async throws -> T {
        let urlRequest = try buildRequest(endpoint, accessToken: accessToken)
        let (data, response) = try await transport.data(for: urlRequest)

        guard let httpResponse = response as? HTTPURLResponse else {
            throw APIError.serverError(statusCode: 0, message: "Invalid response")
        }

        try mapHTTPStatus(httpResponse.statusCode, data: data)

        if T.self == EmptyResponse.self {
            // swiftlint:disable:next force_cast
            return EmptyResponse() as! T
        }

        do {
            return try decoder.decode(T.self, from: data)
        } catch {
            throw APIError.decodingFailed(error.localizedDescription)
        }
    }

    private func buildRequest(
        _ endpoint: APIEndpoint,
        accessToken: String
    ) throws -> URLRequest {
        guard var components = URLComponents(
            url: baseURL.appendingPathComponent(endpoint.path),
            resolvingAgainstBaseURL: false
        ) else {
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
        request.setValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
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
        case 403:
            try mapForbidden(data: data)
        case 404:
            throw APIError.notFound
        default:
            if statusCode >= 400 {
                let message = String(data: data, encoding: .utf8)
                throw APIError.serverError(statusCode: statusCode, message: message)
            }
        }
    }

    private func mapForbidden(data: Data) throws {
        if let body = try? decoder.decode(InsufficientEntitlementBody.self, from: data),
            body.error == "insufficient_entitlement" {
            throw DomainError.insufficientEntitlement(required: body.required)
        }
        let message = String(data: data, encoding: .utf8)
        throw APIError.serverError(statusCode: 403, message: message)
    }
}
