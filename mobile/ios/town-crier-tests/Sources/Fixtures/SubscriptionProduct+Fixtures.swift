import Foundation
import TownCrierDomain

extension SubscriptionProduct {
  static let personal = SubscriptionProduct(
    id: "uk.towncrierapp.personal.monthly",
    displayName: "Personal",
    displayPrice: "£1.99",
    tier: .personal,
    hasFreeTrial: true,
    trialDays: 7,
    period: .monthly,
    price: 1.99
  )

  static let pro = SubscriptionProduct(
    id: "uk.towncrierapp.pro.monthly",
    displayName: "Pro",
    displayPrice: "£4.99",
    tier: .pro,
    period: .monthly,
    price: 4.99
  )

  static let proAnnual = SubscriptionProduct(
    id: "uk.towncrierapp.pro.annual",
    displayName: "Pro Annual",
    displayPrice: "£29.99",
    tier: .pro,
    period: .annual,
    price: 29.99
  )

  static let proLifetime = SubscriptionProduct(
    id: "uk.towncrierapp.pro.lifetime",
    displayName: "Pro Lifetime",
    displayPrice: "£69.99",
    tier: .pro,
    period: .lifetime,
    price: 69.99
  )
}

extension SubscriptionEntitlement {
  static let personalActive = SubscriptionEntitlement(
    tier: .personal,
    expiryDate: Date.distantFuture
  )

  static let proActive = SubscriptionEntitlement(
    tier: .pro,
    expiryDate: Date.distantFuture
  )

  static let proMonthlyActive = SubscriptionEntitlement(
    tier: .pro,
    expiryDate: Date.distantFuture,
    productId: "uk.towncrierapp.pro.monthly"
  )

  static let proAnnualActive = SubscriptionEntitlement(
    tier: .pro,
    expiryDate: Date.distantFuture,
    productId: "uk.towncrierapp.pro.annual"
  )

  static let proLifetime = SubscriptionEntitlement(
    tier: .pro,
    expiryDate: Date.distantFuture,
    productId: "uk.towncrierapp.pro.lifetime",
    isLifetime: true
  )

  static let personalTrial = SubscriptionEntitlement(
    tier: .personal,
    expiryDate: Date.distantFuture,
    isTrialPeriod: true
  )

  static let expired = SubscriptionEntitlement(
    tier: .personal,
    expiryDate: Date.distantPast
  )
}
