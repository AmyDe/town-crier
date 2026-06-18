import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("APIUserProfileRepository data export")
struct APIUserProfileRepositoryExportTests {

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APIUserProfileRepository, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIUserProfileRepository(apiClient: apiClient)
    return (sut, transport)
  }

  // swiftlint:disable force_unwrapping
  private func httpResponse(statusCode: Int) -> HTTPURLResponse {
    HTTPURLResponse(url: baseURL, statusCode: statusCode, httpVersion: nil, headerFields: nil)!
  }
  // swiftlint:enable force_unwrapping

  @Test("exportData sends GET /v1/me/data and returns the raw bytes verbatim")
  func exportData_returnsRawBytes() async throws {
    let json = #"{"profile":{"id":"abc"},"watchZones":[{"id":"z1"}],"notifications":[]}"#
    let bytes = Data(json.utf8)
    let (sut, transport) = makeSUT(responses: [
      (bytes, httpResponse(statusCode: 200))
    ])

    let exported = try await sut.exportData()

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.httpMethod == "GET")
    #expect(request.url?.path().contains("/v1/me/data") == true)
    #expect(exported == bytes, "the response bytes must be preserved exactly, with no re-encoding")
  }

  @Test("exportData with network error throws networkUnavailable")
  func exportData_networkError_throwsNetworkUnavailable() async {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = APIUserProfileRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.exportData()
    }
  }

  @Test("exportData with server error throws serverError")
  func exportData_serverError_throwsServerError() async {
    let (sut, _) = makeSUT(responses: [
      (Data("Internal Server Error".utf8), httpResponse(statusCode: 500))
    ])

    await #expect(
      throws: DomainError.serverError(statusCode: 500, message: "Internal Server Error")
    ) {
      _ = try await sut.exportData()
    }
  }
}
