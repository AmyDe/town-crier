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
}
