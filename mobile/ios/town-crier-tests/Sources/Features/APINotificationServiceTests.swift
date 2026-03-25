import Foundation
import Testing
import TownCrierData
import TownCrierDomain

@Suite("APINotificationService")
struct APINotificationServiceTests {
    // swiftlint:disable:next force_unwrapping
    private let baseURL = URL(string: "https://api.dev.towncrierapp.uk")!

    private func makeTransport(
        statusCode: Int = 204,
        body: String = "{}"
    ) -> StubHTTPTransport {
        let transport = StubHTTPTransport()
        transport.responses = [
            (Data(body.utf8), httpResponse(url: baseURL, statusCode: statusCode)),
        ]
        return transport
    }

    private func makeSUT(
        transport: StubHTTPTransport? = nil,
        session: AuthSession? = .valid
    ) -> (APINotificationService, StubHTTPTransport, SpyAuthenticationService) {
        let transport = transport ?? makeTransport()
        let authSpy = SpyAuthenticationService()
        authSpy.currentSessionResult = session
        let apiClient = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authSpy,
            transport: transport
        )
        let sut = APINotificationService(apiClient: apiClient)
        return (sut, transport, authSpy)
    }

    // MARK: - Register Device Token

    @Test("registerDeviceToken sends PUT /v1/me/device-token with correct body")
    func registerDeviceToken_sendsCorrectRequest() async throws {
        let (sut, transport, _) = makeSUT()

        try await sut.registerDeviceToken("abc123-device-token")

        let request = try #require(transport.requests.first)
        #expect(request.httpMethod == "PUT")
        #expect(request.url?.path().contains("v1/me/device-token") == true)

        let body = try #require(request.httpBody)
        let json = try JSONSerialization.jsonObject(with: body) as? [String: Any]
        #expect(json?["token"] as? String == "abc123-device-token")
        #expect(json?["platform"] as? String == "Ios")
    }

    @Test("registerDeviceToken stores token for later removal")
    func registerDeviceToken_storesToken() async throws {
        let transport = makeTransport()
        // Two responses: one for register, one for remove
        transport.responses.append(
            (Data("{}".utf8), httpResponse(url: baseURL, statusCode: 204))
        )
        let (sut, _, _) = makeSUT(transport: transport)

        try await sut.registerDeviceToken("stored-token-123")
        try await sut.removeDeviceToken()

        let removeRequest = try #require(transport.requests.last)
        #expect(removeRequest.httpMethod == "DELETE")
        #expect(removeRequest.url?.path().contains("stored-token-123") == true)
    }

    // MARK: - Remove Device Token

    @Test("removeDeviceToken sends DELETE /v1/me/device-token/{token}")
    func removeDeviceToken_sendsDeleteRequest() async throws {
        let transport = makeTransport()
        // Two responses: register then remove
        transport.responses.append(
            (Data("{}".utf8), httpResponse(url: baseURL, statusCode: 204))
        )
        let (sut, _, _) = makeSUT(transport: transport)

        try await sut.registerDeviceToken("token-to-remove")
        try await sut.removeDeviceToken()

        let request = try #require(transport.requests.last)
        #expect(request.httpMethod == "DELETE")
        #expect(request.url?.path().contains("v1/me/device-token/token-to-remove") == true)
    }

    @Test("removeDeviceToken with no stored token is a no-op")
    func removeDeviceToken_noStoredToken_doesNothing() async throws {
        let (sut, transport, _) = makeSUT()

        try await sut.removeDeviceToken()

        #expect(transport.requests.isEmpty)
    }
}
