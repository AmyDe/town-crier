import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("APIPlanningApplicationRepository")
struct APIPlanningApplicationRepositoryTests {

  // MARK: - Helpers

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APIPlanningApplicationRepository, SpyAuthenticationService, StubHTTPTransport) {
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

  // MARK: - fetchApplications

  @Test("fetchApplications sends GET /v1/applications with authorityId query param")
  func fetchApplications_sendsCorrectRequest() async throws {
    let json = "[]"
    let authority = LocalAuthority(code: "123", name: "Cambridge")
    let (sut, _, transport) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    _ = try await sut.fetchApplications(for: authority)

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    let url = try #require(request.url)
    #expect(url.path().contains("/v1/applications"))
    #expect(url.query()?.contains("authorityId=123") == true)
  }

  @Test("fetchApplications maps API response to domain models")
  func fetchApplications_mapsToDomainModels() async throws {
    let json = """
      [
          {
              "name": "2026/0042",
              "uid": "app-001",
              "areaName": "Cambridge",
              "areaId": 123,
              "address": "12 Mill Road, Cambridge, CB1 2AD",
              "postcode": "CB1 2AD",
              "description": "Erection of two-storey rear extension",
              "appType": "Full",
              "appState": "Undecided",
              "appSize": null,
              "startDate": "2026-01-15",
              "decidedDate": null,
              "consultedDate": null,
              "longitude": 0.1243,
              "latitude": 52.2043,
              "url": "https://planning.cambridge.gov.uk/2026/0042",
              "link": null,
              "lastDifferent": "2026-01-15T00:00:00+00:00"
          }
      ]
      """
    let authority = LocalAuthority(code: "123", name: "Cambridge")
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.fetchApplications(for: authority)

    #expect(result.count == 1)
    let app = result[0]
    #expect(app.id == PlanningApplicationId("app-001"))
    #expect(app.reference == ApplicationReference("2026/0042"))
    #expect(app.authority.code == "123")
    #expect(app.authority.name == "Cambridge")
    #expect(app.status == ApplicationStatus.undecided)
    #expect(app.description == "Erection of two-storey rear extension")
    #expect(app.address == "12 Mill Road, Cambridge, CB1 2AD")
    let expectedLocation = try Coordinate(latitude: 52.2043, longitude: 0.1243)
    #expect(app.location == expectedLocation)
    #expect(app.portalUrl == URL(string: "https://planning.cambridge.gov.uk/2026/0042"))
  }

  @Test("fetchApplications synthesizes status history from startDate")
  func fetchApplications_synthesizesStatusHistory() async throws {
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
              "appState": "Approved",
              "appSize": null,
              "startDate": "2026-01-15",
              "decidedDate": "2026-03-01",
              "consultedDate": null,
              "longitude": null,
              "latitude": null,
              "url": null,
              "link": null,
              "lastDifferent": "2026-03-01T00:00:00+00:00"
          }
      ]
      """
    let authority = LocalAuthority(code: "123", name: "Cambridge")
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.fetchApplications(for: authority)

    let app = result[0]
    #expect(app.statusHistory.count == 2)
    #expect(app.statusHistory[0].status == ApplicationStatus.undecided)
    #expect(app.statusHistory[1].status == ApplicationStatus.approved)
  }

  @Test("fetchApplications with network error throws networkUnavailable")
  func fetchApplications_networkError_throwsNetworkUnavailable() async throws {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIPlanningApplicationRepository(apiClient: apiClient)
    let authority = LocalAuthority(code: "123", name: "Cambridge")

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.fetchApplications(for: authority)
    }
  }

  @Test("fetchApplications with server error throws serverError not networkUnavailable")
  func fetchApplications_serverError_throwsServerError() async throws {
    let (sut, _, _) = makeSUT(responses: [
      (Data("Bad Request".utf8), httpResponse(statusCode: 400))
    ])
    let authority = LocalAuthority(code: "123", name: "Cambridge")

    await #expect(throws: DomainError.serverError(statusCode: 400, message: "Bad Request")) {
      _ = try await sut.fetchApplications(for: authority)
    }
  }

  @Test("fetchApplications with decoding error throws unexpected not networkUnavailable")
  func fetchApplications_decodingError_throwsUnexpected() async throws {
    // Return invalid JSON that can't be decoded to [PlanningApplicationDTO]
    let (sut, _, _) = makeSUT(responses: [
      (Data("not json".utf8), httpResponse(statusCode: 200))
    ])
    let authority = LocalAuthority(code: "123", name: "Cambridge")

    do {
      _ = try await sut.fetchApplications(for: authority)
      Issue.record("Expected error to be thrown")
    } catch let error as DomainError {
      // Should be .unexpected, not .networkUnavailable
      #expect(error != DomainError.networkUnavailable)
      if case .unexpected = error {
        // correct
      } else if case .serverError = error {
        // also acceptable -- the point is it must NOT be networkUnavailable
      } else {
        Issue.record("Expected .unexpected or .serverError, got \(error)")
      }
    }
  }

  // MARK: - fetchApplication

  @Test("fetchApplication sends GET /v1/applications/{uid}")
  func fetchApplication_sendsCorrectRequest() async throws {
    let json = """
      {
          "name": "2026/0042",
          "uid": "app-001",
          "areaName": "Cambridge",
          "areaId": 123,
          "address": "12 Mill Road, Cambridge, CB1 2AD",
          "postcode": "CB1 2AD",
          "description": "Erection of two-storey rear extension",
          "appType": "Full",
          "appState": "Undecided",
          "appSize": null,
          "startDate": "2026-01-15",
          "decidedDate": null,
          "consultedDate": null,
          "longitude": 0.1243,
          "latitude": 52.2043,
          "url": null,
          "link": null,
          "lastDifferent": "2026-01-15T00:00:00+00:00"
      }
      """
    let (sut, _, transport) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    _ = try await sut.fetchApplication(by: PlanningApplicationId("app-001"))

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    #expect(request.url?.path().contains("/v1/applications/app-001") == true)
  }

  @Test("fetchApplication maps single API response to domain model")
  func fetchApplication_mapsToDomainModel() async throws {
    let json = """
      {
          "name": "2026/0099",
          "uid": "app-002",
          "areaName": "Oxford",
          "areaId": 456,
          "address": "45 High Street, Oxford, OX1 4AS",
          "postcode": "OX1 4AS",
          "description": "Change of use",
          "appType": "Full",
          "appState": "Refused",
          "appSize": null,
          "startDate": "2026-02-01",
          "decidedDate": "2026-03-15",
          "consultedDate": null,
          "longitude": -1.2577,
          "latitude": 51.7520,
          "url": null,
          "link": null,
          "lastDifferent": "2026-03-15T00:00:00+00:00"
      }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let app = try await sut.fetchApplication(by: PlanningApplicationId("app-002"))

    #expect(app.id == PlanningApplicationId("app-002"))
    #expect(app.reference == ApplicationReference("2026/0099"))
    #expect(app.authority == LocalAuthority(code: "456", name: "Oxford"))
    #expect(app.status == ApplicationStatus.refused)
    #expect(app.address == "45 High Street, Oxford, OX1 4AS")
    let expectedLocation = try Coordinate(latitude: 51.7520, longitude: -1.2577)
    #expect(app.location == expectedLocation)
  }

  @Test("fetchApplication with 404 throws applicationNotFound")
  func fetchApplication_notFound_throwsApplicationNotFound() async throws {
    let (sut, _, _) = makeSUT(responses: [
      (Data("null".utf8), httpResponse(statusCode: 404))
    ])

    await #expect(throws: DomainError.applicationNotFound(PlanningApplicationId("missing"))) {
      _ = try await sut.fetchApplication(by: PlanningApplicationId("missing"))
    }
  }

  @Test("fetchApplication with server error throws serverError not networkUnavailable")
  func fetchApplication_serverError_throwsServerError() async throws {
    let (sut, _, _) = makeSUT(responses: [
      (Data("Internal Server Error".utf8), httpResponse(statusCode: 500))
    ])

    await #expect(
      throws: DomainError.serverError(statusCode: 500, message: "Internal Server Error")
    ) {
      _ = try await sut.fetchApplication(by: PlanningApplicationId("app-001"))
    }
  }

  // MARK: - AppState mapping

  @Test(
    "maps known AppState strings to ApplicationStatus",
    arguments: [
      ("Undecided", ApplicationStatus.undecided),
      ("Not Available", ApplicationStatus.notAvailable),
      ("Approved", ApplicationStatus.approved),
      ("Refused", ApplicationStatus.refused),
      ("Withdrawn", ApplicationStatus.withdrawn),
      ("Appealed", ApplicationStatus.appealed),
    ])
  func appStateMapping(appState: String, expected: ApplicationStatus) async throws {
    let json = """
      {
          "name": "REF/001",
          "uid": "id-1",
          "areaName": "Test",
          "areaId": 1,
          "address": "1 Test St",
          "postcode": null,
          "description": "Test",
          "appType": "Full",
          "appState": "\(appState)",
          "appSize": null,
          "startDate": null,
          "decidedDate": null,
          "consultedDate": null,
          "longitude": null,
          "latitude": null,
          "url": null,
          "link": null,
          "lastDifferent": "2026-01-01T00:00:00+00:00"
      }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let app = try await sut.fetchApplication(by: PlanningApplicationId("id-1"))

    #expect(app.status == expected)
  }

  @Test("maps unknown AppState to .unknown")
  func unknownAppState_mapsToUnknown() async throws {
    let json = """
      {
          "name": "REF/001",
          "uid": "id-1",
          "areaName": "Test",
          "areaId": 1,
          "address": "1 Test St",
          "postcode": null,
          "description": "Test",
          "appType": "Full",
          "appState": "Something Unexpected",
          "appSize": null,
          "startDate": null,
          "decidedDate": null,
          "consultedDate": null,
          "longitude": null,
          "latitude": null,
          "url": null,
          "link": null,
          "lastDifferent": "2026-01-01T00:00:00+00:00"
      }
      """
    let (sut, _, _) = makeSUT(responses: [
      (Data(json.utf8), httpResponse(statusCode: 200))
    ])

    let app = try await sut.fetchApplication(by: PlanningApplicationId("id-1"))

    #expect(app.status == .unknown)
  }
}
