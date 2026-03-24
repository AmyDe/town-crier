import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("APIWatchZoneRepository")
struct APIWatchZoneRepositoryTests {

    // MARK: - Helpers

    private let baseURL = URL(string: "https://api.dev.towncrierapp.uk")!

    private func makeSUT(
        responses: [(Data, URLResponse)]
    ) -> (APIWatchZoneRepository, SpyAuthenticationService, StubHTTPTransport) {
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid
        let transport = StubHTTPTransport()
        transport.responses = responses
        let apiClient = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )
        let sut = APIWatchZoneRepository(apiClient: apiClient)
        return (sut, authService, transport)
    }

    private func httpResponse(statusCode: Int) -> HTTPURLResponse {
        HTTPURLResponse(
            url: baseURL,
            statusCode: statusCode,
            httpVersion: nil,
            headerFields: nil
        )!
    }

    // MARK: - save

    @Test("save sends POST /v1/me/watch-zones with correct body")
    func save_sendsCorrectRequest() async throws {
        let zone = WatchZone.cambridge
        let (sut, _, transport) = makeSUT(responses: [
            (Data("{}".utf8), httpResponse(statusCode: 201)),
        ])

        try await sut.save(zone)

        #expect(transport.requests.count == 1)
        let request = transport.requests[0]
        #expect(request.httpMethod == "POST")
        #expect(request.url?.path().contains("/v1/me/watch-zones") == true)

        let body = try #require(request.httpBody)
        let json = try JSONSerialization.jsonObject(with: body) as! [String: Any]
        #expect(json["zoneId"] as? String == "zone-001")
        #expect(json["name"] as? String == "CB1 2AD")
        #expect(json["latitude"] as? Double == 52.2053)
        #expect(json["longitude"] as? Double == 0.1218)
        #expect(json["radiusMetres"] as? Double == 2000)
    }

    @Test("save with network error throws networkUnavailable")
    func save_networkError_throwsNetworkUnavailable() async {
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid
        let transport = StubHTTPTransport()
        transport.error = URLError(.notConnectedToInternet)
        let apiClient = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )
        let sut = APIWatchZoneRepository(apiClient: apiClient)

        await #expect(throws: DomainError.networkUnavailable) {
            try await sut.save(.cambridge)
        }
    }

    // MARK: - loadAll

    @Test("loadAll sends GET /v1/me/watch-zones and maps response to domain models")
    func loadAll_mapsResponseToDomain() async throws {
        let json = """
            {
                "zones": [
                    {
                        "id": "zone-001",
                        "name": "CB1 2AD",
                        "latitude": 52.2053,
                        "longitude": 0.1218,
                        "radiusMetres": 2000,
                        "authorityId": 123
                    }
                ]
            }
            """
        let (sut, _, transport) = makeSUT(responses: [
            (Data(json.utf8), httpResponse(statusCode: 200)),
        ])

        let zones = try await sut.loadAll()

        #expect(transport.requests.count == 1)
        let request = transport.requests[0]
        #expect(request.httpMethod == "GET")
        #expect(request.url?.path().contains("/v1/me/watch-zones") == true)

        #expect(zones.count == 1)
        let zone = zones[0]
        #expect(zone.id == WatchZoneId("zone-001"))
        let expectedPostcode = try Postcode("CB1 2AD")
        #expect(zone.postcode == expectedPostcode)
        let expectedCentre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
        #expect(zone.centre == expectedCentre)
        #expect(zone.radiusMetres == 2000)
    }

    @Test("loadAll returns empty array when no zones")
    func loadAll_emptyZones_returnsEmptyArray() async throws {
        let json = """
            { "zones": [] }
            """
        let (sut, _, _) = makeSUT(responses: [
            (Data(json.utf8), httpResponse(statusCode: 200)),
        ])

        let zones = try await sut.loadAll()

        #expect(zones.isEmpty)
    }

    @Test("loadAll with network error throws networkUnavailable")
    func loadAll_networkError_throwsNetworkUnavailable() async {
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid
        let transport = StubHTTPTransport()
        transport.error = URLError(.notConnectedToInternet)
        let apiClient = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )
        let sut = APIWatchZoneRepository(apiClient: apiClient)

        await #expect(throws: DomainError.networkUnavailable) {
            _ = try await sut.loadAll()
        }
    }

    // MARK: - delete

    @Test("delete sends DELETE /v1/me/watch-zones/{zoneId}")
    func delete_sendsCorrectRequest() async throws {
        let (sut, _, transport) = makeSUT(responses: [
            (Data(), httpResponse(statusCode: 204)),
        ])

        try await sut.delete(WatchZoneId("zone-001"))

        #expect(transport.requests.count == 1)
        let request = transport.requests[0]
        #expect(request.httpMethod == "DELETE")
        #expect(request.url?.path().contains("/v1/me/watch-zones/zone-001") == true)
    }

    @Test("delete with 404 succeeds silently (idempotent)")
    func delete_notFound_succeedsSilently() async throws {
        let (sut, _, _) = makeSUT(responses: [
            (Data("null".utf8), httpResponse(statusCode: 404)),
        ])

        // Should not throw — if the zone is already gone, that's fine
        try await sut.delete(WatchZoneId("nonexistent"))
    }

    @Test("delete with network error throws networkUnavailable")
    func delete_networkError_throwsNetworkUnavailable() async {
        let authService = SpyAuthenticationService()
        authService.currentSessionResult = .valid
        let transport = StubHTTPTransport()
        transport.error = URLError(.notConnectedToInternet)
        let apiClient = URLSessionAPIClient(
            baseURL: baseURL,
            authService: authService,
            transport: transport
        )
        let sut = APIWatchZoneRepository(apiClient: apiClient)

        await #expect(throws: DomainError.networkUnavailable) {
            try await sut.delete(WatchZoneId("zone-001"))
        }
    }
}
