/// Maps subscription tiers to their granted entitlements and quota limits.
///
/// Must remain in sync with the API's `EntitlementMap.cs`.
/// This is a caseless enum because it has no instances — all access is through static methods.
public enum EntitlementMap {

  // MARK: - Entitlement sets per tier

  private static let paidEntitlements: Set<Entitlement> = [
    .statusChangeAlerts,
    .decisionUpdateAlerts,
    .hourlyDigestEmails,
  ]

  // MARK: - Public API

  /// Returns the set of entitlements granted to the given subscription tier.
  public static func entitlements(for tier: SubscriptionTier) -> Set<Entitlement> {
    switch tier {
    case .free:
      return []
    case .personal, .pro:
      return paidEntitlements
    }
  }

  /// Returns the numeric limit for the given quota at the given subscription tier.
  public static func limit(for tier: SubscriptionTier, quota: Quota) -> Int {
    switch (tier, quota) {
    case (.free, .watchZones):
      return 1
    case (.personal, .watchZones):
      return 3
    case (.pro, .watchZones):
      return Int.max
    }
  }

  /// Returns whether the given tier grants the specified entitlement.
  public static func hasEntitlement(_ entitlement: Entitlement, for tier: SubscriptionTier)
    -> Bool {
    entitlements(for: tier).contains(entitlement)
  }

  /// Returns whether the given tier can add another item given the current count for a quota.
  public static func canAdd(for tier: SubscriptionTier, currentCount: Int, quota: Quota) -> Bool {
    currentCount < limit(for: tier, quota: quota)
  }
}
