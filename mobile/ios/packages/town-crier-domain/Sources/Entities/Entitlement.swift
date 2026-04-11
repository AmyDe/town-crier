/// A feature entitlement that a subscription tier may grant.
///
/// Used to parameterise UI gating decisions and the subscription upsell sheet.
/// Must remain in sync with the API's `EntitlementMap`.
public enum Entitlement: String, CaseIterable, Sendable, Identifiable {
  case searchApplications
  case statusChangeAlerts
  case decisionUpdateAlerts
  case hourlyDigestEmails

  public var id: String { rawValue }

  /// User-facing name for the entitlement, used in upsell sheets and settings.
  public var displayName: String {
    switch self {
    case .searchApplications:
      return "Search Applications"
    case .statusChangeAlerts:
      return "Status Change Alerts"
    case .decisionUpdateAlerts:
      return "Decision Update Alerts"
    case .hourlyDigestEmails:
      return "Hourly Digest Emails"
    }
  }

  /// Marketing description explaining the feature, used in the subscription upsell sheet body.
  public var featureDescription: String {
    switch self {
    case .searchApplications:
      return "Search across all planning applications by keyword, address, or reference number."
    case .statusChangeAlerts:
      return "Get notified when a planning application in your watch zone changes status."
    case .decisionUpdateAlerts:
      return "Get notified when a decision is made on a planning application near you."
    case .hourlyDigestEmails:
      return "Receive an hourly email digest summarising new planning activity in your watch zones."
    }
  }
}
