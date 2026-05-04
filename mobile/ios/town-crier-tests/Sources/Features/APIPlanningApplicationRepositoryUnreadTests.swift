import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// Decoding tests for the `latestUnreadEvent` field added to the per-zone
/// applications response (tc-1nsa.3 server-side, tc-1nsa.8 client-side).
///
/// The field is optional/nullable: older builds may omit it entirely, the
/// server returns `null` when no notification exists strictly after the
/// user's `lastReadAt` watermark, and a populated event drives the
/// muted/saturated styling of the row's status pill plus the
/// `recent-activity` sort order.
///
/// Spec: `docs/specs/notifications-unread-watermark.md#api-augment-applications`.
@Suite("APIPlanningApplicationRepository — latestUnreadEvent decoding")
struct APIPlanningApplicationRepositoryUnreadTests {

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  // swiftlint:disable:next force_try
  private let testZone = try! WatchZone(
    id: WatchZoneId("zone-123"),
    name: "Cambridge",
    centre: Coordinate(latitude: 52.2053, longitude: 0.1218),
    radiusMetres: 2000,
    authorityId: 123
  )

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> APIPlanningApplicationRepository {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    return APIPlanningApplicationRepository(apiClient: apiClient)
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

  private func payload(latestUnreadJSON: String) -> Data {
    let json = """
      [
          {
              "name": "2026/0042",
              "uid": "app-001",
              "areaName": "Cambridge",
              "areaId": 123,
              "address": "12 Mill Road",
              "postcode": null,
              "description": "Extension",
              "appType": "Full",
              "appState": "Undecided",
              "appSize": null,
              "startDate": "2026-01-15",
              "decidedDate": null,
              "consultedDate": null,
              "longitude": null,
              "latitude": null,
              "url": null,
              "link": null,
              "lastDifferent": "2026-01-15T00:00:00+00:00"
              \(latestUnreadJSON)
          }
      ]
      """
    return Data(json.utf8)
  }

  @Test("decodes a populated DecisionUpdate event")
  func decodesDecisionUpdateEvent() async throws {
    let body = payload(
      latestUnreadJSON: """
        ,"latestUnreadEvent": {
          "type": "DecisionUpdate",
          "decision": "Permitted",
          "createdAt": "2026-03-01T12:34:56Z"
        }
        """
    )
    let sut = makeSUT(responses: [(body, httpResponse(statusCode: 200))])

    let result = try await sut.fetchApplications(for: testZone)

    let event = try #require(result.first?.latestUnreadEvent)
    #expect(event.type == "DecisionUpdate")
    #expect(event.decision == "Permitted")
  }

  @Test("decodes a NewApplication event with null decision")
  func decodesNewApplicationEvent() async throws {
    let body = payload(
      latestUnreadJSON: """
        ,"latestUnreadEvent": {
          "type": "NewApplication",
          "decision": null,
          "createdAt": "2026-03-01T12:34:56Z"
        }
        """
    )
    let sut = makeSUT(responses: [(body, httpResponse(statusCode: 200))])

    let result = try await sut.fetchApplications(for: testZone)

    let event = try #require(result.first?.latestUnreadEvent)
    #expect(event.type == "NewApplication")
    #expect(event.decision == nil)
  }

  @Test("createdAt is decoded as an ISO-8601 instant")
  func decodesCreatedAtAsISO8601Instant() async throws {
    let body = payload(
      latestUnreadJSON: """
        ,"latestUnreadEvent": {
          "type": "NewApplication",
          "decision": null,
          "createdAt": "2026-03-01T12:34:56Z"
        }
        """
    )
    let sut = makeSUT(responses: [(body, httpResponse(statusCode: 200))])

    let result = try await sut.fetchApplications(for: testZone)

    let event = try #require(result.first?.latestUnreadEvent)
    let formatter = ISO8601DateFormatter()
    let expected = try #require(formatter.date(from: "2026-03-01T12:34:56Z"))
    #expect(event.createdAt == expected)
  }

  @Test("explicit null decodes to nil")
  func explicitNullDecodesToNil() async throws {
    let body = payload(latestUnreadJSON: ",\"latestUnreadEvent\": null")
    let sut = makeSUT(responses: [(body, httpResponse(statusCode: 200))])

    let result = try await sut.fetchApplications(for: testZone)

    #expect(result.first?.latestUnreadEvent == nil)
  }

  @Test("omitted field decodes to nil (older builds)")
  func omittedFieldDecodesToNil() async throws {
    let body = payload(latestUnreadJSON: "")
    let sut = makeSUT(responses: [(body, httpResponse(statusCode: 200))])

    let result = try await sut.fetchApplications(for: testZone)

    #expect(result.first?.latestUnreadEvent == nil)
  }
}
