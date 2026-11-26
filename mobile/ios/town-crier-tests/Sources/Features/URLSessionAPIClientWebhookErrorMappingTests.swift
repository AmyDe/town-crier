import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("URLSessionAPIClient webhook error mapping")
struct URLSessionAPIClientWebhookErrorMappingTests {

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func requestFailure(statusCode: Int, body: String) async -> (any Error)? {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = [(Data(body.utf8), httpResponse(url: baseURL, statusCode: statusCode))]
    let sut = URLSessionAPIClient(
      baseURL: baseURL, authService: authService, transport: transport)

    do {
      let _: EmptyResponse = try await sut.request(.get("/v1/me/webhook"))
      return nil
    } catch {
      return error
    }
  }

  @Test(
    "webhook error codes map to their DomainError",
    arguments: [
      (400, "webhook_url_invalid", DomainError.webhookURLInvalid),
      (400, "webhook_format_invalid", DomainError.webhookFormatInvalid),
      (403, "webhook_requires_pro", DomainError.webhookRequiresPro),
      (429, "webhook_test_cooldown", DomainError.webhookTestCooldown),
    ])
  func webhookCode_mapsToDomainError(statusCode: Int, code: String, expected: DomainError) async {
    let failure = await requestFailure(
      statusCode: statusCode, body: #"{"error":"\#(code)","message":"nope"}"#)

    #expect(failure as? DomainError == expected)
  }

  @Test("403 insufficient_entitlement still maps to insufficientEntitlement")
  func forbidden_insufficientEntitlement_isUnchanged() async {
    let failure = await requestFailure(
      statusCode: 403, body: #"{"error":"insufficient_entitlement","required":"pro"}"#)

    #expect(failure as? DomainError == .insufficientEntitlement(required: "pro"))
  }

  @Test("429 with another body throws the generic serverError")
  func tooManyRequests_otherBody_throwsServerError() async {
    let body = #"{"error":"rate_limited"}"#

    let failure = await requestFailure(statusCode: 429, body: body)

    guard case .serverError(let statusCode, let message) = failure as? APIError else {
      Issue.record("Expected APIError.serverError, got \(String(describing: failure))")
      return
    }
    #expect(statusCode == 429)
    #expect(message == body)
  }

  @Test(
    "the debug log preview never contains a webhook response body",
    arguments: ["/v1/me/webhook", "/v1/me/webhook/secret"])
  func logPreview_webhookPath_isRedacted(path: String) {
    let body = Data(#"{"secret":"whsec_abc"}"#.utf8)

    let preview = URLSessionAPIClient.logPreview(of: body, path: path)

    #expect(preview == "<redacted>")
  }

  @Test("the debug log preview keeps other response bodies")
  func logPreview_otherPath_keepsBody() {
    let preview = URLSessionAPIClient.logPreview(of: Data("{}".utf8), path: "/v1/me")

    #expect(preview == "{}")
  }
}
