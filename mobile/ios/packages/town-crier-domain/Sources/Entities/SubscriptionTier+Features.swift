extension SubscriptionTier {
  /// User-facing feature highlights for this tier, shown on the paywall, derived
  /// from ``WatchZoneLimits`` and ``EntitlementMap``.
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
    // The API gates webhooks on the Pro tier itself, not on an entitlement.
    if self == .pro {
      highlights.append("Webhooks (JSON, Slack or Discord)")
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
