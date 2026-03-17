/// A subscription product available for purchase.
public struct SubscriptionProduct: Equatable, Hashable, Sendable {
    public let id: String
    public let displayName: String
    public let displayPrice: String
    public let tier: SubscriptionTier
    public let hasFreeTrial: Bool
    public let trialDays: Int

    public init(
        id: String,
        displayName: String,
        displayPrice: String,
        tier: SubscriptionTier,
        hasFreeTrial: Bool = false,
        trialDays: Int = 0
    ) {
        self.id = id
        self.displayName = displayName
        self.displayPrice = displayPrice
        self.tier = tier
        self.hasFreeTrial = hasFreeTrial
        self.trialDays = trialDays
    }
}
