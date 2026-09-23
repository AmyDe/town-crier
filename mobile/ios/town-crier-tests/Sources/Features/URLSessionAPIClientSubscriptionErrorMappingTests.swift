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

/// 409 subscription-conflict error mapping (tc-k42ce.2, GH#1165). Split out of
/// `URLSessionAPIClientWatchZoneErrorMappingTests` since it is a distinct
/// endpoint family, mirroring the existing per-concern test-file split.
@Suite("URLSessionAPIClient -- Subscription Error Mapping")
struct URLSessionAPIClientSubscriptionErrorMappingTests {
  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(responses: [(Data, URLResponse)]) -> URLSessionAPIClient {
    let transport = StubHTTPTransport()
    transport.responses = responses
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    return URLSessionAPIClient(baseURL: baseURL, authService: authService, transport: transport)
  }

  @Test("409 with transaction_already_claimed body throws DomainError.transactionAlreadyClaimed")
  func transactionAlreadyClaimedResponse() async throws {
    let body =
      #"{"error":"transaction_already_claimed","message":"This transaction belongs to another account."}"#
    let sut = makeSUT(responses: [(Data(body.utf8), httpResponse(url: baseURL, statusCode: 409))])

    await #expect(throws: DomainError.transactionAlreadyClaimed) {
      let _: TestResponse = try await sut.request(
        .post("/v1/subscriptions/verify", body: TestBody(title: "x")))
    }
  }

  @Test("409 with zone_name_taken body still throws DomainError.watchZoneNameTaken")
  func zoneNameTakenResponseUnaffected() async throws {
    let body =
      #"{"error":"zone_name_taken","message":"You already have a watch zone with this name."}"#
    let sut = makeSUT(responses: [(Data(body.utf8), httpResponse(url: baseURL, statusCode: 409))])

    await #expect(throws: DomainError.watchZoneNameTaken) {
      let _: TestResponse = try await sut.request(.post("/watch-zones", body: TestBody(title: "x")))
    }
  }
}
