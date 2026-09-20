import Foundation

/// The user's current subscription entitlement state.
///
/// A lifetime purchase never expires, so its `expiryDate` is `Date.distantFuture`
/// and `isLifetime` is true.
public struct SubscriptionEntitlement: Equatable, Sendable {
  public let tier: SubscriptionTier
  public let expiryDate: Date
  public let isTrialPeriod: Bool
  public let productId: String?
  public let isLifetime: Bool

  public init(
    tier: SubscriptionTier,
    expiryDate: Date,
    isTrialPeriod: Bool = false,
    productId: String? = nil,
    isLifetime: Bool = false
  ) {
    self.tier = tier
    self.expiryDate = expiryDate
    self.isTrialPeriod = isTrialPeriod
    self.productId = productId
    self.isLifetime = isLifetime
  }

  public var isActive: Bool {
    expiryDate > Date()
  }
}
