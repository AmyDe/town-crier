import Foundation
import Testing

@testable import TownCrierData

@Suite("OfferCodeError")
struct OfferCodeErrorTests {
  @Test("maps invalid_code_format to .invalidFormat")
  func invalidCodeFormat_mapsToInvalidFormat() {
    let error = OfferCodeError(serverErrorCode: "invalid_code_format")
    #expect(error == .invalidFormat)
  }

  @Test("maps invalid_code to .notFound")
  func invalidCode_mapsToNotFound() {
    let error = OfferCodeError(serverErrorCode: "invalid_code")
    #expect(error == .notFound)
  }

  @Test("maps code_already_redeemed to .alreadyRedeemed")
  func codeAlreadyRedeemed_mapsToAlreadyRedeemed() {
    let error = OfferCodeError(serverErrorCode: "code_already_redeemed")
    #expect(error == .alreadyRedeemed)
  }

  @Test("maps already_subscribed to .alreadySubscribed")
  func alreadySubscribed_mapsToAlreadySubscribed() {
    let error = OfferCodeError(serverErrorCode: "already_subscribed")
    #expect(error == .alreadySubscribed)
  }

  @Test("maps unknown code to .network with context")
  func unknownCode_mapsToNetworkWithContext() {
    let error = OfferCodeError(serverErrorCode: "mystery_meat")

    guard case let .network(message) = error else {
      Issue.record("Expected .network case, got \(error)")
      return
    }
    #expect(message.contains("mystery_meat"))
  }
}
