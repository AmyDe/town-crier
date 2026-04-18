import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("HttpOfferCodeService")
struct HttpOfferCodeServiceTests {
  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (HttpOfferCodeService, StubHTTPTransport) {
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = .valid
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = URLSessionAPIClient(
      baseURL: baseURL,
      authService: authService,
      transport: transport
    )
    let sut = HttpOfferCodeService(apiClient: apiClient)
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

  @Test("redeem sends POST /v1/offer-codes/redeem with the raw code in the body")
  func redeem_sendsCorrectRequest() async throws {
    let body = """
      {
        "tier": "pro",
        "expiresAt": "2026-05-18T12:00:00Z"
      }
      """
    let (sut, transport) = makeSUT(responses: [
      (Data(body.utf8), httpResponse(statusCode: 200))
    ])

    _ = try await sut.redeem(code: "A7KM-ZQR3-FNXP")

    let request = try #require(transport.requests.first)
    #expect(request.httpMethod == "POST")
    #expect(request.url?.path().contains("v1/offer-codes/redeem") == true)

    let httpBody = try #require(request.httpBody)
    let json = try JSONSerialization.jsonObject(with: httpBody) as? [String: Any]
    #expect(json?["code"] as? String == "A7KM-ZQR3-FNXP")
  }

  @Test("redeem decodes tier and expiresAt from the response")
  func redeem_decodesResponse() async throws {
    let body = """
      {
        "tier": "pro",
        "expiresAt": "2026-05-18T12:00:00Z"
      }
      """
    let (sut, _) = makeSUT(responses: [
      (Data(body.utf8), httpResponse(statusCode: 200))
    ])

    let redemption = try await sut.redeem(code: "A7KMZQR3FNXP")

    #expect(redemption.tier == .pro)
    let expected = ISO8601DateFormatter().date(from: "2026-05-18T12:00:00Z")
    #expect(redemption.expiresAt == expected)
  }

  @Test("redeem throws .network when tier rawValue is unknown")
  func redeem_unknownTier_throwsNetworkError() async throws {
    let body = """
      {
        "tier": "platinum",
        "expiresAt": "2026-05-18T12:00:00Z"
      }
      """
    let (sut, _) = makeSUT(responses: [
      (Data(body.utf8), httpResponse(statusCode: 200))
    ])

    await #expect(throws: OfferCodeError.self) {
      _ = try await sut.redeem(code: "A7KMZQR3FNXP")
    }
  }

  // MARK: - Error mapping

  @Test("redeem maps 400 invalid_code_format body to .invalidFormat")
  func redeem_400InvalidFormat_throwsInvalidFormat() async throws {
    let errorBody = Data(#"{"error":"invalid_code_format"}"#.utf8)
    let (sut, _) = makeSUT(responses: [
      (errorBody, httpResponse(statusCode: 400))
    ])

    await #expect(throws: OfferCodeError.invalidFormat) {
      _ = try await sut.redeem(code: "bad")
    }
  }

  @Test("redeem maps 404 invalid_code body to .notFound")
  func redeem_404InvalidCode_throwsNotFound() async throws {
    let errorBody = Data(#"{"error":"invalid_code"}"#.utf8)
    let (sut, _) = makeSUT(responses: [
      (errorBody, httpResponse(statusCode: 404))
    ])

    await #expect(throws: OfferCodeError.notFound) {
      _ = try await sut.redeem(code: "A7KMZQR3FNXP")
    }
  }

  @Test("redeem maps 409 code_already_redeemed body to .alreadyRedeemed")
  func redeem_409AlreadyRedeemed_throwsAlreadyRedeemed() async throws {
    let errorBody = Data(#"{"error":"code_already_redeemed"}"#.utf8)
    let (sut, _) = makeSUT(responses: [
      (errorBody, httpResponse(statusCode: 409))
    ])

    await #expect(throws: OfferCodeError.alreadyRedeemed) {
      _ = try await sut.redeem(code: "A7KMZQR3FNXP")
    }
  }

  @Test("redeem maps 409 already_subscribed body to .alreadySubscribed")
  func redeem_409AlreadySubscribed_throwsAlreadySubscribed() async throws {
    let errorBody = Data(#"{"error":"already_subscribed"}"#.utf8)
    let (sut, _) = makeSUT(responses: [
      (errorBody, httpResponse(statusCode: 409))
    ])

    await #expect(throws: OfferCodeError.alreadySubscribed) {
      _ = try await sut.redeem(code: "A7KMZQR3FNXP")
    }
  }

  @Test("redeem falls back to .network when 4xx body is unparseable")
  func redeem_4xxUnparseableBody_throwsNetwork() async throws {
    let errorBody = Data("not json".utf8)
    let (sut, _) = makeSUT(responses: [
      (errorBody, httpResponse(statusCode: 400))
    ])

    do {
      _ = try await sut.redeem(code: "A7KMZQR3FNXP")
      Issue.record("Expected redeem to throw")
    } catch let error as OfferCodeError {
      guard case .network = error else {
        Issue.record("Expected .network, got \(error)")
        return
      }
    }
  }
}
