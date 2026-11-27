import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("APIWebhookRepository")
struct APIWebhookRepositoryTests {

  // MARK: - Helpers

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    statusCode: Int, body: String
  ) -> (APIWebhookRepository, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = [(Data(body.utf8), httpResponse(url: baseURL, statusCode: statusCode))]
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL, authService: authService, transport: transport)
    return (APIWebhookRepository(apiClient: apiClient), transport)
  }

  private func webhookJSON(
    format: String = "slack", lastFailureReason: String = "\"http_status\"", extra: String = ""
  ) -> String {
    """
    {
      "url": "https://example.com/hook",
      "format": "\(format)",
      "status": "disabled",
      "disabledReason": "failures",
      "paused": true,
      "createdAt": "2026-11-26T14:00:00Z",
      "lastSuccessAt": "2026-11-26T15:00:00Z",
      "lastFailureAt": "2026-11-26T16:00:00Z",
      "lastFailureReason": \(lastFailureReason),
      "lastFailureStatus": 500\(extra)
    }
    """
  }

  private let emptyHistoryJSON = """
    {
      "url": "https://example.com/hook",
      "format": "json",
      "status": "active",
      "disabledReason": null,
      "paused": false,
      "createdAt": "2026-11-26T14:00:00Z",
      "lastSuccessAt": null,
      "lastFailureAt": null,
      "lastFailureReason": null,
      "lastFailureStatus": null
    }
    """

  private func date(_ epochSeconds: TimeInterval) -> Date {
    Date(timeIntervalSince1970: epochSeconds)
  }

  private func expectRequest(
    _ transport: StubHTTPTransport, method: String, path: String
  ) {
    #expect(transport.requests.count == 1)
    #expect(transport.requests.first?.httpMethod == method)
    #expect(transport.requests.first?.url?.path() == path)
  }

  // MARK: - fetch

  @Test("fetch maps every field of the response")
  func fetch_mapsEveryField() async throws {
    let (sut, transport) = makeSUT(statusCode: 200, body: webhookJSON())

    let webhook = try await sut.fetch()

    expectRequest(transport, method: "GET", path: "/v1/me/webhook")
    #expect(
      webhook
        == Webhook(
          url: "https://example.com/hook",
          format: .slack,
          status: .disabled,
          disabledReason: .failures,
          isPaused: true,
          createdAt: date(1_795_701_600),
          lastSuccessAt: date(1_795_705_200),
          lastFailureAt: date(1_795_708_800),
          lastFailureReason: .httpStatus,
          lastFailureStatus: 500
        ))
  }

  @Test("fetch maps null fields to nil")
  func fetch_mapsNulls() async throws {
    let (sut, _) = makeSUT(statusCode: 200, body: emptyHistoryJSON)

    let webhook = try #require(try await sut.fetch())

    #expect(webhook.format == .json)
    #expect(webhook.status == .active)
    #expect(webhook.disabledReason == nil)
    #expect(!webhook.isPaused)
    #expect(webhook.lastSuccessAt == nil)
    #expect(webhook.lastFailureAt == nil)
    #expect(webhook.lastFailureReason == nil)
    #expect(webhook.lastFailureStatus == nil)
  }

  @Test(
    "fetch decodes each failure reason",
    arguments: [
      ("gone", WebhookFailureReason.gone),
      ("http_status", .httpStatus),
      ("blocked_address", .blockedAddress),
      ("timeout", .timeout),
      ("tls", .tls),
      ("connection", .connection),
    ])
  func fetch_decodesFailureReason(wire: String, expected: WebhookFailureReason) async throws {
    let (sut, _) = makeSUT(
      statusCode: 200, body: webhookJSON(lastFailureReason: "\"\(wire)\""))

    let webhook = try await sut.fetch()

    #expect(webhook?.lastFailureReason == expected)
  }

  @Test("fetch decodes an unknown lastFailureReason as nil")
  func fetch_unknownFailureReason_isNil() async throws {
    let (sut, _) = makeSUT(
      statusCode: 200, body: webhookJSON(lastFailureReason: "\"cosmic_rays\""))

    let webhook = try #require(try await sut.fetch())

    #expect(webhook.lastFailureReason == nil)
    #expect(webhook.lastFailureStatus == 500)
  }

  @Test("fetch returns nil on 404")
  func fetch_notFound_returnsNil() async throws {
    let (sut, _) = makeSUT(statusCode: 404, body: #"{"error":"webhook_not_found"}"#)

    let webhook = try await sut.fetch()

    #expect(webhook == nil)
  }

  @Test("fetch throws a domain error on 500")
  func fetch_serverError_throws() async {
    let (sut, _) = makeSUT(statusCode: 500, body: "boom")

    await #expect(throws: DomainError.serverError(statusCode: 500, message: "boom")) {
      _ = try await sut.fetch()
    }
  }

  // MARK: - save

  @Test("save sends PUT with url and format and returns the secret of a create")
  func save_create_returnsSecret() async throws {
    let (sut, transport) = makeSUT(
      statusCode: 201, body: webhookJSON(extra: #", "secret": "whsec_abc""#))

    let result = try await sut.save(url: "https://example.com/hook", format: .slack)

    expectRequest(transport, method: "PUT", path: "/v1/me/webhook")
    let body = try #require(transport.requests.first?.httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: String])
    #expect(json == ["url": "https://example.com/hook", "format": "slack"])
    #expect(result.secret == "whsec_abc")
    #expect(result.webhook.url == "https://example.com/hook")
  }

  @Test("save sends the discord wire value")
  func save_discord_sendsWireValue() async throws {
    let (sut, transport) = makeSUT(
      statusCode: 200, body: webhookJSON(format: "discord"))

    let result = try await sut.save(url: "https://example.com/hook", format: .discord)

    let body = try #require(transport.requests.first?.httpBody)
    let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: String])
    #expect(json["format"] == "discord")
    #expect(result.webhook.format == .discord)
  }

  @Test("save returns no secret for a replace")
  func save_replace_returnsNilSecret() async throws {
    let (sut, _) = makeSUT(statusCode: 200, body: emptyHistoryJSON)

    let result = try await sut.save(url: "https://example.com/hook", format: .json)

    #expect(result.secret == nil)
    #expect(result.webhook.format == .json)
  }

  @Test("save throws webhookURLInvalid for a rejected URL")
  func save_invalidURL_throws() async {
    let (sut, _) = makeSUT(statusCode: 400, body: #"{"error":"webhook_url_invalid"}"#)

    await #expect(throws: DomainError.webhookURLInvalid) {
      _ = try await sut.save(url: "http://example.com", format: .json)
    }
  }

  // MARK: - remove

  @Test("remove sends DELETE and succeeds on 204")
  func remove_succeeds() async throws {
    let (sut, transport) = makeSUT(statusCode: 204, body: "")

    try await sut.remove()

    expectRequest(transport, method: "DELETE", path: "/v1/me/webhook")
  }

  // MARK: - rotateSecret

  @Test("rotateSecret sends POST and returns the new secret")
  func rotateSecret_returnsSecret() async throws {
    let (sut, transport) = makeSUT(statusCode: 200, body: #"{"secret":"whsec_new"}"#)

    let secret = try await sut.rotateSecret()

    expectRequest(transport, method: "POST", path: "/v1/me/webhook/secret")
    #expect(secret == "whsec_new")
  }

  @Test("rotateSecret turns 404 into webhookNotFound")
  func rotateSecret_notFound_throwsWebhookNotFound() async {
    let (sut, _) = makeSUT(statusCode: 404, body: #"{"error":"webhook_not_found"}"#)

    await #expect(throws: DomainError.webhookNotFound) {
      _ = try await sut.rotateSecret()
    }
  }

  // MARK: - sendTest

  @Test("sendTest maps a delivered result")
  func sendTest_delivered() async throws {
    let (sut, transport) = makeSUT(
      statusCode: 200, body: #"{"delivered":true,"status":200,"failureReason":null}"#)

    let result = try await sut.sendTest()

    expectRequest(transport, method: "POST", path: "/v1/me/webhook/test")
    #expect(result == WebhookTestResult(delivered: true, status: 200, failureReason: nil))
  }

  @Test("sendTest maps a failed result with no status")
  func sendTest_notDelivered() async throws {
    let (sut, _) = makeSUT(
      statusCode: 200, body: #"{"delivered":false,"status":null,"failureReason":"timeout"}"#)

    let result = try await sut.sendTest()

    #expect(result == WebhookTestResult(delivered: false, status: nil, failureReason: .timeout))
  }

  @Test("sendTest turns 404 into webhookNotFound")
  func sendTest_notFound_throwsWebhookNotFound() async {
    let (sut, _) = makeSUT(statusCode: 404, body: #"{"error":"webhook_not_found"}"#)

    await #expect(throws: DomainError.webhookNotFound) {
      _ = try await sut.sendTest()
    }
  }
}
