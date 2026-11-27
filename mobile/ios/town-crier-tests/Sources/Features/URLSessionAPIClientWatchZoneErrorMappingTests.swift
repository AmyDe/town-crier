import Foundation
import Testing
import TownCrierData
import TownCrierDomain

private struct TestResponse: Decodable, Equatable, Sendable {
  let id: String
  let name: String
}

private struct TestBody: Codable, Sendable {
  let title: String
}

/// 400/409 watch-zone error mapping (tc-9oyhw, GH#1085). Split out of
/// `URLSessionAPIClientTests` to keep that file under the `file_length`
/// SwiftLint threshold, mirroring the existing split into
/// `URLSessionAPIClientTokenRefreshTests`/`URLSessionAPIClientPagedTests`.
@Suite("URLSessionAPIClient -- Watch-Zone Error Mapping")
struct URLSessionAPIClientWatchZoneErrorMappingTests {
  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(responses: [(Data, URLResponse)]) -> URLSessionAPIClient {
    let transport = StubHTTPTransport()
    transport.responses = responses
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    return URLSessionAPIClient(baseURL: baseURL, authService: authService, transport: transport)
  }

  @Test("400 with boundary_too_large body throws DomainError.invalidWatchZoneBoundaryTooLarge")
  func boundaryTooLargeResponse() async throws {
    let body =
      #"{"error":"boundary_too_large","message":"The boundary is too large for your subscription tier."}"#
    let sut = makeSUT(responses: [(Data(body.utf8), httpResponse(url: baseURL, statusCode: 400))])

    await #expect(throws: DomainError.invalidWatchZoneBoundaryTooLarge) {
      let _: TestResponse = try await sut.request(.post("/watch-zones", body: TestBody(title: "x")))
    }
  }

  @Test("409 with zone_name_taken body throws DomainError.watchZoneNameTaken")
  func zoneNameTakenResponse() async throws {
    let body =
      #"{"error":"zone_name_taken","message":"You already have a watch zone with this name."}"#
    let sut = makeSUT(responses: [(Data(body.utf8), httpResponse(url: baseURL, statusCode: 409))])

    await #expect(throws: DomainError.watchZoneNameTaken) {
      let _: TestResponse = try await sut.request(.post("/watch-zones", body: TestBody(title: "x")))
    }
  }

  @Test("400 with unrecognized error code falls back to APIError.serverError")
  func unrecognizedBadRequestFallsBackToServerError() async throws {
    let body = #"{"error":"validation_failed","message":"Something else is wrong."}"#
    let sut = makeSUT(responses: [(Data(body.utf8), httpResponse(url: baseURL, statusCode: 400))])

    await #expect(throws: APIError.self) {
      let _: TestResponse = try await sut.request(.post("/watch-zones", body: TestBody(title: "x")))
    }
  }

  @Test("409 with malformed body falls back to APIError.serverError")
  func malformedConflictFallsBackToServerError() async throws {
    let body = "not json"
    let sut = makeSUT(responses: [(Data(body.utf8), httpResponse(url: baseURL, statusCode: 409))])

    await #expect(throws: APIError.self) {
      let _: TestResponse = try await sut.request(.post("/watch-zones", body: TestBody(title: "x")))
    }
  }
}
