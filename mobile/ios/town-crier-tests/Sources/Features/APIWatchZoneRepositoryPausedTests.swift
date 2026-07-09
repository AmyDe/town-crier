import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// Paused-zone wire-format tests for `APIWatchZoneRepository` (GH#889 P2).
///
/// A zone is marked `paused` server-side when a subscription downgrade left
/// the user over their tier's quota — purely a display flag, never computed
/// or mutated by the client.
@Suite("APIWatchZoneRepository — paused zones")
struct APIWatchZoneRepositoryPausedTests {

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APIWatchZoneRepository, StubHTTPTransport) {
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
    return (sut, transport)
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

  @Test("loadAll decodes paused: true from the API response")
  func loadAll_pausedTrue_decodesPausedField() async throws {
    let json = """
      {
          "zones": [
              {
                  "id": "zone-001",
                  "name": "CB1 2AD",
                  "latitude": 52.2053,
                  "longitude": 0.1218,
                  "radiusMetres": 2000,
                  "authorityId": 123,
                  "paused": true
              }
          ]
      }
      """
    let (sut, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let zones = try await sut.loadAll()

    #expect(zones.count == 1)
    #expect(zones[0].paused)
  }

  @Test("loadAll defaults paused to false when the field is absent (back-compat)")
  func loadAll_missingPaused_defaultsToFalse() async throws {
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
    let (sut, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let zones = try await sut.loadAll()

    #expect(zones.count == 1)
    #expect(!zones[0].paused)
  }
}
