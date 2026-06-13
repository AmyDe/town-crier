import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// Watch-zone quota (tc-gpjk): the create endpoint's only 403 is
/// "quota exceeded", so the repository normalises a create 403 to
/// `DomainError.insufficientEntitlement(required:)` — giving the UI the
/// "Upgrade Required" path instead of a generic "Server Error". This covers
/// BOTH forms the client may surface: a plain quota 403 (which the client maps
/// to `serverError(statusCode: 403, _)`) and an entitlement-shaped 403 body
/// (already mapped to `.insufficientEntitlement`). `update` (PATCH) is left
/// unchanged because quota only applies to create.
@Suite("APIWatchZoneRepository — create quota 403")
struct APIWatchZoneRepositoryQuotaTests {

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> APIWatchZoneRepository {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    return APIWatchZoneRepository(apiClient: apiClient)
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

  @Test("save maps a plain quota 403 to insufficientEntitlement(required: personal)")
  func save_quota403_mapsToInsufficientEntitlement() async {
    let body = "Watch zone quota exceeded. Upgrade your subscription for more zones."
    let sut = makeSUT(responses: [
      (Data(body.utf8), httpResponse(statusCode: 403))
    ])

    await #expect(throws: DomainError.insufficientEntitlement(required: "personal")) {
      try await sut.save(.cambridge)
    }
  }

  @Test("save passes through an entitlement-shaped 403 as insufficientEntitlement")
  func save_entitlement403_passesThrough() async {
    let body = #"{"error":"insufficient_entitlement","required":"pro"}"#
    let sut = makeSUT(responses: [
      (Data(body.utf8), httpResponse(statusCode: 403))
    ])

    await #expect(throws: DomainError.insufficientEntitlement(required: "pro")) {
      try await sut.save(.cambridge)
    }
  }

  @Test("update with a 403 is left unchanged — quota only applies to create")
  func update_403_throwsServerError() async {
    let body = "Forbidden"
    let sut = makeSUT(responses: [
      (Data(body.utf8), httpResponse(statusCode: 403))
    ])

    await #expect(throws: DomainError.serverError(statusCode: 403, message: body)) {
      try await sut.update(.cambridge)
    }
  }
}
