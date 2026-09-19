import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("StoreKitSubscriptionService — product mapping")
struct StoreKitSubscriptionServiceMappingTests {

  @Test func productIds_includeAllFourProducts() {
    #expect(
      Set(StoreKitSubscriptionService.productIds)
        == [
          "uk.towncrierapp.personal.monthly",
          "uk.towncrierapp.pro.monthly",
          "uk.towncrierapp.pro.annual",
          "uk.towncrierapp.pro.lifetime",
        ]
    )
  }

  @Test(
    arguments: [
      ("uk.towncrierapp.personal.monthly", SubscriptionTier.personal),
      ("uk.towncrierapp.pro.monthly", SubscriptionTier.pro),
      ("uk.towncrierapp.pro.annual", SubscriptionTier.pro),
      ("uk.towncrierapp.pro.lifetime", SubscriptionTier.pro),
    ]
  )
  func tier_mapsEveryProductId(productId: String, expected: SubscriptionTier) {
    #expect(StoreKitSubscriptionService.tier(forProductId: productId) == expected)
  }

  @Test func tier_isNilForUnknownProductId() {
    #expect(StoreKitSubscriptionService.tier(forProductId: "uk.towncrierapp.unknown") == nil)
  }

  @Test(
    arguments: [
      ("uk.towncrierapp.personal.monthly", SubscriptionPeriod.monthly),
      ("uk.towncrierapp.pro.monthly", SubscriptionPeriod.monthly),
      ("uk.towncrierapp.pro.annual", SubscriptionPeriod.annual),
      ("uk.towncrierapp.pro.lifetime", SubscriptionPeriod.lifetime),
    ]
  )
  func period_mapsEveryProductId(productId: String, expected: SubscriptionPeriod) {
    #expect(StoreKitSubscriptionService.period(forProductId: productId) == expected)
  }

  @Test func period_isNilForUnknownProductId() {
    #expect(StoreKitSubscriptionService.period(forProductId: "uk.towncrierapp.unknown") == nil)
  }

  @Test func sorted_ordersByTierThenMonthlyAnnualLifetime() {
    let shuffled: [SubscriptionProduct] = [.proLifetime, .pro, .proAnnual, .personal]

    let sorted = StoreKitSubscriptionService.sorted(shuffled)

    #expect(sorted == [.personal, .pro, .proAnnual, .proLifetime])
  }

  @Test func entitlement_forNonConsumable_isLifetimeWithDistantFutureExpiry() {
    let entitlement = StoreKitSubscriptionService.entitlement(
      productId: "uk.towncrierapp.pro.lifetime",
      expirationDate: nil,
      isIntroductoryOffer: false,
      isNonConsumable: true
    )

    #expect(entitlement.tier == .pro)
    #expect(entitlement.isLifetime)
    #expect(entitlement.expiryDate == .distantFuture)
    #expect(entitlement.productId == "uk.towncrierapp.pro.lifetime")
    #expect(!entitlement.isTrialPeriod)
  }

  @Test func entitlement_forAutoRenewable_carriesProductIdAndExpiry() {
    let expiry = Date(timeIntervalSince1970: 2_000_000_000)

    let entitlement = StoreKitSubscriptionService.entitlement(
      productId: "uk.towncrierapp.pro.annual",
      expirationDate: expiry,
      isIntroductoryOffer: false,
      isNonConsumable: false
    )

    #expect(entitlement.tier == .pro)
    #expect(!entitlement.isLifetime)
    #expect(entitlement.expiryDate == expiry)
    #expect(entitlement.productId == "uk.towncrierapp.pro.annual")
  }

  @Test func entitlement_forIntroductoryOffer_isTrial() {
    let entitlement = StoreKitSubscriptionService.entitlement(
      productId: "uk.towncrierapp.personal.monthly",
      expirationDate: .distantFuture,
      isIntroductoryOffer: true,
      isNonConsumable: false
    )

    #expect(entitlement.isTrialPeriod)
  }

  @Test func preferred_whenNoCurrent_returnsCandidate() {
    let result = StoreKitSubscriptionService.preferredEntitlement(
      current: nil, candidate: .proMonthlyActive
    )

    #expect(result == .proMonthlyActive)
  }

  @Test func preferred_whenSameTier_prefersLifetimeCandidate() {
    let result = StoreKitSubscriptionService.preferredEntitlement(
      current: .proMonthlyActive, candidate: .proLifetime
    )

    #expect(result == .proLifetime)
  }

  @Test func preferred_whenSameTier_keepsLifetimeCurrent() {
    let result = StoreKitSubscriptionService.preferredEntitlement(
      current: .proLifetime, candidate: .proAnnualActive
    )

    #expect(result == .proLifetime)
  }

  @Test func preferred_whenCandidateTierIsHigher_prefersCandidate() {
    let result = StoreKitSubscriptionService.preferredEntitlement(
      current: .personalActive, candidate: .proMonthlyActive
    )

    #expect(result == .proMonthlyActive)
  }

  @Test func preferred_whenCurrentTierIsHigher_keepsCurrent() {
    let result = StoreKitSubscriptionService.preferredEntitlement(
      current: .proMonthlyActive, candidate: .personalActive
    )

    #expect(result == .proMonthlyActive)
  }
}
