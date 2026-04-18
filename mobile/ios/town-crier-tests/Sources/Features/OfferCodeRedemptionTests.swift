import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("OfferCodeRedemption")
struct OfferCodeRedemptionTests {
  @Test("stores tier and expiresAt")
  func initStoresFields() {
    let expiresAt = Date(timeIntervalSince1970: 1_800_000_000)
    let redemption = OfferCodeRedemption(tier: .pro, expiresAt: expiresAt)

    #expect(redemption.tier == .pro)
    #expect(redemption.expiresAt == expiresAt)
  }

  @Test("is equatable by value")
  func equatableByValue() {
    let expiresAt = Date(timeIntervalSince1970: 1_800_000_000)
    let personalOne = OfferCodeRedemption(tier: .personal, expiresAt: expiresAt)
    let personalTwo = OfferCodeRedemption(tier: .personal, expiresAt: expiresAt)
    let proOne = OfferCodeRedemption(tier: .pro, expiresAt: expiresAt)

    #expect(personalOne == personalTwo)
    #expect(personalOne != proOne)
  }
}
