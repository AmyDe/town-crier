import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("APIPostcodeGeocoder")
struct APIPostcodeGeocoderTests {

    // MARK: - Helpers

    // swiftlint:disable:next force_unwrapping
    private let baseURL = URL(string: "https://api.dev.towncrierapp.uk")!

    private func makeSUT(
        responses: [(Data, URLResponse)]
    ) -> (APIPostcodeGeocoder, SpyAuthenticationService, StubHTTPTransport) {
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid
        let transport = StubHTTPTransport()
        transport.responses = responses
        let apiClient = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )
        let sut = APIPostcodeGeocoder(apiClient: apiClient)
        return (sut, authService, transport)
    }

    // swiftlint:disable force_unwrapping
    private func httpResponse(statusCode: Int) -> HTTPURLResponse {
        HTTPURLResponse(
            url: baseURL,
            statusCode: statusCode,
            httpVersion: nil,
            headerFields: nil
        )!
    }
    // swiftlint:enable force_unwrapping

    // MARK: - Success

    @Test("geocode sends GET /v1/geocode/{postcode} and returns mapped Coordinate")
    func geocode_success_returnsMappedCoordinate() async throws {
        let json = #"{"coordinates":{"latitude":52.2053,"longitude":0.1218}}"#
        let (sut, _, transport) = makeSUT(responses: [
            (Data(json.utf8), httpResponse(statusCode: 200)),
        ])

        let postcode = try Postcode("CB2 1TN")
        let result = try await sut.geocode(postcode)

        let expected = try Coordinate(latitude: 52.2053, longitude: 0.1218)
        #expect(result == expected)
        #expect(transport.requests.count == 1)
        let request = transport.requests[0]
        #expect(request.url?.path().contains("/v1/geocode/CB2%201TN") == true
            || request.url?.path().contains("/v1/geocode/CB2 1TN") == true)
    }

    // MARK: - Error mapping

    @Test("geocode with 404 throws geocodingFailed")
    func geocode_notFound_throwsGeocodingFailed() async throws {
        let errorJson = #"{"message":"Postcode 'ZZ9 9ZZ' could not be geocoded."}"#
        let (sut, _, _) = makeSUT(responses: [
            (Data(errorJson.utf8), httpResponse(statusCode: 404)),
        ])

        let postcode = try Postcode("ZZ9 9ZZ")
        await #expect(throws: DomainError.self) {
            _ = try await sut.geocode(postcode)
        }
    }

    @Test("geocode with server error throws geocodingFailed")
    func geocode_serverError_throwsGeocodingFailed() async throws {
        let (sut, _, _) = makeSUT(responses: [
            (Data(), httpResponse(statusCode: 500)),
        ])

        let postcode = try Postcode("CB2 1TN")
        await #expect(throws: DomainError.self) {
            _ = try await sut.geocode(postcode)
        }
    }

    @Test("geocode with network error throws networkUnavailable")
    func geocode_networkError_throwsNetworkUnavailable() async throws {
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid
        let transport = StubHTTPTransport()
        transport.error = URLError(.notConnectedToInternet)
        let apiClient = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )
        let sut = APIPostcodeGeocoder(apiClient: apiClient)

        let postcode = try Postcode("CB2 1TN")
        await #expect(throws: DomainError.networkUnavailable) {
            _ = try await sut.geocode(postcode)
        }
    }
}
