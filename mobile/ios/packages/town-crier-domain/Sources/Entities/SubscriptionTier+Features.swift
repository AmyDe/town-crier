extension SubscriptionTier {
  /// Concise, user-facing feature highlights for this tier, shown on the paywall.
  ///
  /// Derived from ``WatchZoneLimits`` and ``EntitlementMap`` so the copy stays in
  /// sync with the zone limits and entitlements the app actually enforces. Order
  /// runs from quotas (zones, radius) to alerting.
  ///
  /// Push and email are framed as two channels for the same alerts rather than
  /// separate features, so the paid tiers collapse to a single alerts line.
  public var featureHighlights: [String] {
    let limits = WatchZoneLimits(tier: self)
    var highlights = [
      Self.zoneCountHighlight(maxZones: limits.maxZones),
      Self.radiusHighlight(maxRadiusMetres: limits.maxRadiusMetres),
    ]
    if EntitlementMap.hasEntitlement(.statusChangeAlerts, for: self) {
      highlights.append("Status & decision alerts by push and email")
    } else {
      highlights.append("Weekly email summary")
    }
    return highlights
  }

  private static func zoneCountHighlight(maxZones: Int) -> String {
    switch maxZones {
    case Int.max:
      return "Unlimited watch zones"
    case 1:
      return "1 watch zone"
    default:
      return "\(maxZones) watch zones"
    }
  }

  private static func radiusHighlight(maxRadiusMetres: Double) -> String {
    let kilometres = Int(maxRadiusMetres / 1000)
    return "Zones up to \(kilometres) km radius"
  }
}
