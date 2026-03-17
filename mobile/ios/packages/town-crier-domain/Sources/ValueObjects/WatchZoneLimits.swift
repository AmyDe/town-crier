/// Encodes the watch zone limits for a subscription tier.
public struct WatchZoneLimits: Equatable, Sendable {
    public let tier: SubscriptionTier
    public let maxZones: Int
    public let maxRadiusMetres: Double

    public init(tier: SubscriptionTier) {
        self.tier = tier
        switch tier {
        case .free:
            maxZones = 1
            maxRadiusMetres = 2000
        case .personal:
            maxZones = 1
            maxRadiusMetres = 5000
        case .pro:
            maxZones = .max
            maxRadiusMetres = 10000
        }
    }

    /// Whether the user can add another zone given their current count.
    public func canAddZone(currentCount: Int) -> Bool {
        currentCount < maxZones
    }

    /// Clamps a radius to the tier's maximum.
    public func clampRadius(_ radius: Double) -> Double {
        min(radius, maxRadiusMetres)
    }

    /// Predefined radius options available for this tier.
    public var availableRadiusOptions: [Double] {
        let allOptions: [Double] = [500, 1000, 1500, 2000, 3000, 5000, 7500, 10000]
        return allOptions.filter { $0 <= maxRadiusMetres }
    }
}
