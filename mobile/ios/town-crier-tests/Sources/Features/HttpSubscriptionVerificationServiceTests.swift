import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("HttpSubscriptionVerificationService")
struct HttpSubscriptionVerificationServiceTests {
  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (HttpSubscriptionVerificationService, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = HttpSubscriptionVerificationService(apiClient: apiClient)
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

  // MARK: - Happy path

  @Test("verify POSTs /v1/subscriptions/verify with the signed transaction in the body")
  func verify_sendsCorrectRequest() async throws {
    let body = """
      {
        "tier": "Personal",
        "subscriptionExpiry": "2026-05-11T00:00:00+00:00",
        "entitlements": ["StatusChangeAlerts"],
        "watchZoneLimit": 3
      }
      """
    let (sut, transport) = makeSUT(responses: [
      (Data(body.utf8), httpResponse(statusCode: 200))
    ])

    _ = try await sut.verify(signedTransaction: "compact.jws.string")

    let request = try #require(transport.requests.first)
    #expect(request.httpMethod == "POST")
    #expect(request.url?.path().contains("v1/subscriptions/verify") == true)

    let httpBody = try #require(request.httpBody)
    let json = try JSONSerialization.jsonObject(with: httpBody) as? [String: Any]
    #expect(json?["signedTransaction"] as? String == "compact.jws.string")
  }

  @Test("verify decodes the tier and expiry from a 200 response")
  func verify_decodesResponse() async throws {
    let body = """
      {
        "tier": "Personal",
        "subscriptionExpiry": "2026-05-11T00:00:00+00:00",
        "entitlements": ["StatusChangeAlerts","DecisionUpdateAlerts"],
        "watchZoneLimit": 3
      }
      """
    let (sut, _) = makeSUT(responses: [
      (Data(body.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.verify(signedTransaction: "compact.jws.string")

    #expect(result.tier == .personal)
    #expect(result.watchZoneLimit == 3)
  }

  @Test("verify maps a null subscriptionExpiry to a nil expiry")
  func verify_nullExpiry_decodesAsNil() async throws {
    let body = """
      {
        "tier": "Free",
        "subscriptionExpiry": null,
        "entitlements": [],
        "watchZoneLimit": 1
      }
      """
    let (sut, _) = makeSUT(responses: [
      (Data(body.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.verify(signedTransaction: "compact.jws.string")

    #expect(result.tier == .free)
    #expect(result.subscriptionExpiry == nil)
  }

  // MARK: - Error mapping

  @Test("verify surfaces a 401 invalid_transaction as an error")
  func verify_401_throws() async throws {
    let errorBody = Data(#"{"error":"invalid_transaction"}"#.utf8)
    let (sut, _) = makeSUT(responses: [
      (errorBody, httpResponse(statusCode: 401)),
      (errorBody, httpResponse(statusCode: 401)),
    ])

    await #expect(throws: (any Error).self) {
      _ = try await sut.verify(signedTransaction: "bad.jws")
    }
  }

  // MARK: - Restore

  @Test("verifyRestore POSTs /v1/subscriptions/verify with a signedTransactions list")
  func verifyRestore_sendsTransactionList() async throws {
    let body = """
      {
        "tier": "Pro",
        "subscriptionExpiry": "2026-06-11T00:00:00+00:00",
        "entitlements": ["StatusChangeAlerts"],
        "watchZoneLimit": 2147483647
      }
      """
    let (sut, transport) = makeSUT(responses: [
      (Data(body.utf8), httpResponse(statusCode: 200))
    ])

    _ = try await sut.verifyRestore(signedTransactions: ["jws.one", "jws.two"])

    let request = try #require(transport.requests.first)
    #expect(request.httpMethod == "POST")
    #expect(request.url?.path().contains("v1/subscriptions/verify") == true)

    let httpBody = try #require(request.httpBody)
    let json = try JSONSerialization.jsonObject(with: httpBody) as? [String: Any]
    #expect(json?["signedTransactions"] as? [String] == ["jws.one", "jws.two"])
    #expect(json?["signedTransaction"] == nil)
  }

  @Test("verifyRestore decodes the server-resolved tier from a 200 response")
  func verifyRestore_decodesResponse() async throws {
    let body = """
      {
        "tier": "Pro",
        "subscriptionExpiry": "2026-06-11T00:00:00+00:00",
        "entitlements": ["StatusChangeAlerts","DecisionUpdateAlerts"],
        "watchZoneLimit": 2147483647
      }
      """
    let (sut, _) = makeSUT(responses: [
      (Data(body.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.verifyRestore(signedTransactions: ["jws.one"])

    #expect(result.tier == .pro)
    #expect(result.watchZoneLimit == 2_147_483_647)
  }

  @Test("verifyRestore decodes a Free tier when only expired transactions are restored")
  func verifyRestore_expiredTransactions_decodesAsFree() async throws {
    let body = """
      {
        "tier": "Free",
        "subscriptionExpiry": null,
        "entitlements": [],
        "watchZoneLimit": 1
      }
      """
    let (sut, _) = makeSUT(responses: [
      (Data(body.utf8), httpResponse(statusCode: 200))
    ])

    let result = try await sut.verifyRestore(signedTransactions: ["expired.jws"])

    #expect(result.tier == .free)
    #expect(result.subscriptionExpiry == nil)
  }

  @Test("verifyRestore surfaces a 401 invalid_transaction as an error")
  func verifyRestore_401_throws() async throws {
    let errorBody = Data(#"{"error":"invalid_transaction"}"#.utf8)
    let (sut, _) = makeSUT(responses: [
      (errorBody, httpResponse(statusCode: 401)),
      (errorBody, httpResponse(statusCode: 401)),
    ])

    await #expect(throws: (any Error).self) {
      _ = try await sut.verifyRestore(signedTransactions: ["tampered.jws"])
    }
  }
}
