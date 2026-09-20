/// Port for subscription product listing, purchasing, and entitlement resolution.
public protocol SubscriptionService: Sendable {
  /// Returns available subscription products with localised pricing.
  func availableProducts() async throws -> [SubscriptionProduct]

  /// Purchases the product and returns the resulting entitlement.
  func purchase(_ productId: String) async throws -> SubscriptionEntitlement

  /// Restores previously purchased subscriptions. Returns nil if none found.
  func restorePurchases() async throws -> SubscriptionEntitlement?

  /// Returns the current active entitlement, or nil if on the free tier.
  func currentEntitlement() async -> SubscriptionEntitlement?

  /// Returns true when an active auto-renewing subscription is still set to renew,
  /// or when its renewal state cannot be read. Used after a lifetime purchase to
  /// decide whether to ask the user to cancel their old subscription.
  func hasActiveAutoRenewingSubscription() async -> Bool
}
