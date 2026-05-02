import TownCrierDomain

/// Proactive UI gating helper that ViewModels use to check entitlements and quotas
/// without a network round-trip.
///
/// Wraps a ``SubscriptionTier`` and delegates to ``EntitlementMap`` for all
/// business logic. ViewModels receive this from the session tier at construction
/// time, enabling them to disable/badge features before the user taps.
///
/// Usage:
/// ```swift
/// let gate = FeatureGate(tier: session.subscriptionTier)
/// if gate.hasEntitlement(.statusChangeAlerts) { ... }
/// if gate.shouldShowUpgradeBadge(for: .watchZones, currentCount: zones.count) { ... }
/// ```
public struct FeatureGate: Equatable, Sendable {
  /// The subscription tier this gate was initialised with.
  public let tier: SubscriptionTier

  public init(tier: SubscriptionTier) {
    self.tier = tier
  }

  // MARK: - Entitlement checks

  /// Returns whether the current tier grants the specified entitlement.
  public func hasEntitlement(_ entitlement: Entitlement) -> Bool {
    EntitlementMap.hasEntitlement(entitlement, for: tier)
  }

  // MARK: - Quota checks

  /// Returns whether the current tier allows adding another item for the given quota.
  public func canAdd(quota: Quota, currentCount: Int) -> Bool {
    EntitlementMap.canAdd(for: tier, currentCount: currentCount, quota: quota)
  }

  // MARK: - Upgrade badge

  /// Returns whether an "Upgrade" badge should be shown for a feature entitlement.
  ///
  /// True when the current tier does not grant the entitlement.
  public func shouldShowUpgradeBadge(for entitlement: Entitlement) -> Bool {
    !hasEntitlement(entitlement)
  }

  /// Returns whether an "Upgrade" badge should be shown for a quota-gated action.
  ///
  /// True when the current count has reached the tier's limit (i.e. cannot add more).
  public func shouldShowUpgradeBadge(for quota: Quota, currentCount: Int) -> Bool {
    !canAdd(quota: quota, currentCount: currentCount)
  }
}
