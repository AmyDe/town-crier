import Foundation

/// The user's current subscription entitlement state.
public struct SubscriptionEntitlement: Equatable, Sendable {
  public let tier: SubscriptionTier
  public let expiryDate: Date
  public let isTrialPeriod: Bool

  public init(
    tier: SubscriptionTier,
    expiryDate: Date,
    isTrialPeriod: Bool = false
  ) {
    self.tier = tier
    self.expiryDate = expiryDate
    self.isTrialPeriod = isTrialPeriod
  }

  public var isActive: Bool {
    expiryDate > Date()
  }
}
