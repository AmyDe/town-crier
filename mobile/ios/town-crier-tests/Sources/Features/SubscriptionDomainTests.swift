import Foundation
import Testing
import TownCrierDomain

@Suite("Subscription domain types")
struct SubscriptionDomainTests {

  @Test func period_ordersMonthlyThenAnnualThenLifetime() {
    #expect(SubscriptionPeriod.monthly < .annual)
    #expect(SubscriptionPeriod.annual < .lifetime)
    #expect(SubscriptionPeriod.allCases.sorted() == [.monthly, .annual, .lifetime])
  }

  @Test func product_defaultsToMonthlyWithZeroPriceAndPoundsCurrency() {
    let product = SubscriptionProduct(
      id: "id",
      displayName: "Name",
      displayPrice: "£1.00",
      tier: .personal
    )

    #expect(product.period == .monthly)
    #expect(product.price == 0)
    #expect(product.currencyCode == "GBP")
  }

  @Test func product_carriesPeriodPriceAndCurrency() {
    let product = SubscriptionProduct(
      id: "id",
      displayName: "Name",
      displayPrice: "€29.99",
      tier: .pro,
      period: .annual,
      price: 29.99,
      currencyCode: "EUR"
    )

    #expect(product.period == .annual)
    #expect(product.price == 29.99)
    #expect(product.currencyCode == "EUR")
  }

  @Test func entitlement_defaultsToNoProductAndNotLifetime() {
    let entitlement = SubscriptionEntitlement(tier: .pro, expiryDate: .distantFuture)

    #expect(entitlement.productId == nil)
    #expect(!entitlement.isLifetime)
  }

  @Test func lifetimeEntitlement_isActiveThroughDistantFutureExpiry() {
    let entitlement = SubscriptionEntitlement.proLifetime

    #expect(entitlement.isLifetime)
    #expect(entitlement.productId == "uk.towncrierapp.pro.lifetime")
    #expect(entitlement.isActive)
  }
}
