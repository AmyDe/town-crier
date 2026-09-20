import Foundation

/// A subscription product available for purchase.
///
/// `displayPrice` is the storefront-formatted string. `price` is the numeric
/// amount in `currencyCode`, for price arithmetic such as annual savings.
public struct SubscriptionProduct: Equatable, Hashable, Sendable {
  public let id: String
  public let displayName: String
  public let displayPrice: String
  public let tier: SubscriptionTier
  public let hasFreeTrial: Bool
  public let trialDays: Int
  public let period: SubscriptionPeriod
  public let price: Decimal
  public let currencyCode: String

  public init(
    id: String,
    displayName: String,
    displayPrice: String,
    tier: SubscriptionTier,
    hasFreeTrial: Bool = false,
    trialDays: Int = 0,
    period: SubscriptionPeriod = .monthly,
    price: Decimal = 0,
    currencyCode: String = "GBP"
  ) {
    self.id = id
    self.displayName = displayName
    self.displayPrice = displayPrice
    self.tier = tier
    self.hasFreeTrial = hasFreeTrial
    self.trialDays = trialDays
    self.period = period
    self.price = price
    self.currencyCode = currencyCode
  }
}
