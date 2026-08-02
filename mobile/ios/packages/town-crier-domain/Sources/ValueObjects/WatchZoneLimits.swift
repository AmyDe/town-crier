/// Encodes the watch zone limits for a subscription tier.
///
/// Zone count limits delegate to ``EntitlementMap`` so there is a single source of truth
/// that stays in sync with the API's `EntitlementMap.cs`.
public struct WatchZoneLimits: Equatable, Sendable {
  public let tier: SubscriptionTier
  public let maxZones: Int
  public let maxRadiusMetres: Double
  /// Whether this tier can draw a custom-shape (polygon) zone rather than
  /// only a circle (GH#1031). Mirrors the server's
  /// `SubscriptionTier.AllowsCustomBoundary()` (`t.IsPaid()`).
  public let allowsCustomBoundary: Bool

  public init(tier: SubscriptionTier) {
    self.tier = tier
    maxZones = EntitlementMap.limit(for: tier, quota: .watchZones)
    switch tier {
    case .free:
      maxRadiusMetres = 2000
      allowsCustomBoundary = false
    case .personal:
      maxRadiusMetres = 5000
      allowsCustomBoundary = true
    case .pro:
      maxRadiusMetres = 10000
      allowsCustomBoundary = true
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
}
