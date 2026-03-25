import Foundation
import Testing
import TownCrierData
import TownCrierDomain

// MARK: - Test helpers

private struct TestResponse: Decodable, Equatable, Sendable {
    let id: String
    let name: String
}

private struct TestBody: Codable, Sendable {
    let title: String
}

// MARK: - Tests

@Suite("URLSessionAPIClient")
struct URLSessionAPIClientTests {
    // swiftlint:disable:next force_unwrapping
    private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

    // MARK: - Authenticated requests

    @Test("GET request includes Bearer token from current session")
    func getRequestIncludesBearerToken() async throws {
        let transport = StubHTTPTransport()
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid

        let json = #"{"id":"1","name":"Test"}"#
        transport.responses = [
            (Data(json.utf8), httpResponse(url: baseURL, statusCode: 200)),
        ]

        let sut = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )

        let _: TestResponse = try await sut.request(.get("/applications"))

        #expect(transport.requests.count == 1)
        let request = transport.requests[0]
        #expect(request.value(forHTTPHeaderField: "Authorization") == "Bearer test-access-token")
        #expect(request.url?.absoluteString == "https://api-dev.towncrierapp.uk/applications")
        #expect(request.httpMethod == "GET")
    }

    // MARK: - JSON decoding

    @Test("Successful JSON response is decoded to expected type")
    func successfulJsonDecoding() async throws {
        let transport = StubHTTPTransport()
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid

        let json = #"{"id":"42","name":"Extension"}"#
        transport.responses = [
            (Data(json.utf8), httpResponse(url: baseURL, statusCode: 200)),
        ]

        let sut = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )

        let result: TestResponse = try await sut.request(.get("/applications"))

        #expect(result == TestResponse(id: "42", name: "Extension"))
    }

    // MARK: - Token refresh on 401

    @Test("On 401, refreshes session and retries the request")
    func refreshesOnUnauthorized() async throws {
        let transport = StubHTTPTransport()
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid
        authService.refreshSessionResult = .success(.valid)

        let json = #"{"id":"1","name":"Test"}"#
        transport.responses = [
            (Data(), httpResponse(url: baseURL, statusCode: 401)),
            (Data(json.utf8), httpResponse(url: baseURL, statusCode: 200)),
        ]

        let sut = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )

        let result: TestResponse = try await sut.request(.get("/applications"))

        #expect(result == TestResponse(id: "1", name: "Test"))
        #expect(authService.refreshSessionCallCount == 1)
        #expect(transport.requests.count == 2)
    }

    @Test("On 401 when refresh fails, throws sessionExpired")
    func sessionExpiredWhenRefreshFails() async throws {
        let transport = StubHTTPTransport()
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid
        authService.refreshSessionResult = .failure(DomainError.sessionExpired)

        transport.responses = [
            (Data(), httpResponse(url: baseURL, statusCode: 401)),
        ]

        let sut = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )

        await #expect(throws: DomainError.sessionExpired) {
            let _: TestResponse = try await sut.request(.get("/applications"))
        }
    }

    // MARK: - No session (unauthenticated)

    @Test("When no session exists, throws sessionExpired")
    func noSessionThrowsSessionExpired() async throws {
        let transport = StubHTTPTransport()
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = nil

        let sut = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )

        await #expect(throws: DomainError.sessionExpired) {
            let _: TestResponse = try await sut.request(.get("/applications"))
        }
    }

    // MARK: - HTTP error mapping

    @Test("404 response throws APIError.notFound")
    func notFoundResponse() async throws {
        let transport = StubHTTPTransport()
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid

        transport.responses = [
            (Data(), httpResponse(url: baseURL, statusCode: 404)),
        ]

        let sut = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )

        await #expect(throws: APIError.self) {
            let _: TestResponse = try await sut.request(.get("/applications/999"))
        }
    }

    @Test("500 response throws APIError.serverError")
    func serverErrorResponse() async throws {
        let transport = StubHTTPTransport()
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid

        transport.responses = [
            (Data(), httpResponse(url: baseURL, statusCode: 500)),
        ]

        let sut = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )

        await #expect(throws: APIError.self) {
            let _: TestResponse = try await sut.request(.get("/applications"))
        }
    }

    // MARK: - Network errors

    @Test("Network error maps to DomainError.networkUnavailable")
    func networkErrorMapping() async throws {
        let transport = StubHTTPTransport()
        transport.responses = [] // will throw URLError
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid

        let sut = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )

        await #expect(throws: DomainError.networkUnavailable) {
            let _: TestResponse = try await sut.request(.get("/applications"))
        }
    }

    // MARK: - POST with body

    @Test("POST request encodes body as JSON")
    func postRequestEncodesBody() async throws {
        let transport = StubHTTPTransport()
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid

        let json = #"{"id":"new","name":"Created"}"#
        transport.responses = [
            (Data(json.utf8), httpResponse(url: baseURL, statusCode: 201)),
        ]

        let sut = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )

        let body = TestBody(title: "New Application")
        let result: TestResponse = try await sut.request(.post("/applications", body: body))

        #expect(result == TestResponse(id: "new", name: "Created"))
        let request = transport.requests[0]
        #expect(request.httpMethod == "POST")
        #expect(request.value(forHTTPHeaderField: "Content-Type") == "application/json")

        let sentBody = try #require(request.httpBody)
        let decoded = try JSONDecoder().decode(TestBody.self, from: sentBody)
        #expect(decoded.title == "New Application")
    }

    // MARK: - DELETE with no response body

    @Test("DELETE request sends correct method and path")
    func deleteRequest() async throws {
        let transport = StubHTTPTransport()
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid

        transport.responses = [
            (Data("{}".utf8), httpResponse(url: baseURL, statusCode: 204)),
        ]

        let sut = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )

        let _: EmptyResponse = try await sut.request(.delete("/watch-zones/123"))

        let request = transport.requests[0]
        #expect(request.httpMethod == "DELETE")
        #expect(request.url?.path() == "/watch-zones/123")
    }

    // MARK: - Query parameters

    @Test("Request includes query parameters in URL")
    func queryParameters() async throws {
        let transport = StubHTTPTransport()
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid

        let json = #"{"id":"1","name":"Test"}"#
        transport.responses = [
            (Data(json.utf8), httpResponse(url: baseURL, statusCode: 200)),
        ]

        let sut = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )

        let endpoint = APIEndpoint.get("/applications", query: [
            URLQueryItem(name: "authority", value: "camden"),
            URLQueryItem(name: "status", value: "pending"),
        ])

        let _: TestResponse = try await sut.request(endpoint)

        let url = try #require(transport.requests[0].url)
        let components = try #require(URLComponents(url: url, resolvingAgainstBaseURL: false))
        let queryItems = try #require(components.queryItems)
        #expect(queryItems.contains(URLQueryItem(name: "authority", value: "camden")))
        #expect(queryItems.contains(URLQueryItem(name: "status", value: "pending")))
    }
}
