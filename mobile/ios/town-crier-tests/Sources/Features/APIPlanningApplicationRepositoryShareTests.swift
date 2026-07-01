import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// Wire tests for GH #738 Slice 4: the additive `authoritySlug` field on the
/// detail JSON and the anonymous by-slug read that backs inbound Universal Link
/// resolution.
@Suite("APIPlanningApplicationRepository — share")
struct APIPlanningApplicationRepositoryShareTests {

  // MARK: - Helpers

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
  ) -> (APIPlanningApplicationRepository, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIPlanningApplicationRepository(apiClient: apiClient)
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

  // MARK: - authoritySlug decode

  @Test("fetchApplication decodes the additive authoritySlug field onto the authority")
  func fetchApplication_decodesAuthoritySlug() async throws {
    let json = """
      {
          "name": "Kingston/25/02755/CLC",
          "uid": "app-003",
          "areaName": "Kingston upon Thames",
          "areaId": 789,
          "authoritySlug": "kingston",
          "address": "1 Market Place, Kingston, KT1 1JS",
          "postcode": "KT1 1JS",
          "description": "Certificate of lawfulness",
          "appType": "Full",
          "appState": "Undecided",
          "appSize": null,
          "startDate": "2026-02-01",
          "decidedDate": null,
          "consultedDate": null,
          "longitude": null,
          "latitude": null,
          "url": null,
          "link": null,
          "lastDifferent": "2026-02-01T00:00:00+00:00"
      }
      """
    let (sut, _) = makeSUT(responses: [(Data(json.utf8), httpResponse(statusCode: 200))])

    let app = try await sut.fetchApplication(
      by: PlanningApplicationId(authority: "789", name: "Kingston/25/02755/CLC")
    )

    #expect(app.authority.slug == "kingston")
  }

  @Test("fetchApplications leaves authority slug nil when the field is absent (list endpoints)")
  func fetchApplications_absentAuthoritySlug_isNil() async throws {
    // The list/zone endpoints omit `authoritySlug` (server-side `omitempty`), so
    // it must decode as optional and stay nil rather than fail decoding.
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
          }
      ]
      """
    let (sut, _) = makeSUT(responses: [(Data(json.utf8), httpResponse(statusCode: 200))])

    let result = try await sut.fetchApplications(for: testZone)

    #expect(result[0].authority.slug == nil)
  }

  // MARK: - fetchApplication(bySlug:ref:)

  @Test("fetchApplication(bySlug:) sends GET /v1/applications/by-slug/{slug}/{ref}")
  func fetchApplicationBySlug_sendsCorrectRequest() async throws {
    let json = """
      {
          "name": "Kingston/25/02755/CLC",
          "uid": "app-003",
          "areaName": "Kingston upon Thames",
          "areaId": 789,
          "authoritySlug": "kingston",
          "address": "1 Market Place, Kingston, KT1 1JS",
          "postcode": "KT1 1JS",
          "description": "Certificate of lawfulness",
          "appType": "Full",
          "appState": "Undecided",
          "appSize": null,
          "startDate": "2026-02-01",
          "decidedDate": null,
          "consultedDate": null,
          "longitude": null,
          "latitude": null,
          "url": null,
          "link": null,
          "lastDifferent": "2026-02-01T00:00:00+00:00"
      }
      """
    let (sut, transport) = makeSUT(responses: [(Data(json.utf8), httpResponse(statusCode: 200))])

    let app = try await sut.fetchApplication(bySlug: "kingston", ref: "Kingston/25/02755/CLC")

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    // The ref's slashes pass through into the path verbatim, exactly like the
    // by-id read interpolates `id.name`.
    #expect(
      request.url?.path().contains("/v1/applications/by-slug/kingston/Kingston/25/02755/CLC")
        == true)
    #expect(app.id == PlanningApplicationId(authority: "789", name: "Kingston/25/02755/CLC"))
    #expect(app.authority.slug == "kingston")
  }

  @Test("fetchApplication(bySlug:) with 404 throws applicationNotFound")
  func fetchApplicationBySlug_notFound_throwsApplicationNotFound() async throws {
    let (sut, _) = makeSUT(responses: [(Data("null".utf8), httpResponse(statusCode: 404))])

    await #expect(throws: DomainError.self) {
      _ = try await sut.fetchApplication(bySlug: "kingston", ref: "Kingston/25/GONE")
    }
  }

  @Test("fetchApplication(bySlug:) with server error throws serverError not networkUnavailable")
  func fetchApplicationBySlug_serverError_throwsServerError() async throws {
    let (sut, _) = makeSUT(responses: [
      (Data("Internal Server Error".utf8), httpResponse(statusCode: 500))
    ])

    await #expect(
      throws: DomainError.serverError(statusCode: 500, message: "Internal Server Error")
    ) {
      _ = try await sut.fetchApplication(bySlug: "kingston", ref: "Kingston/25/02755/CLC")
    }
  }
}
