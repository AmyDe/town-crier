import TownCrierDomain

/// Extracts deep link destinations from push notification payloads.
public enum NotificationPayloadParser {
  public static func parseDeepLink(from userInfo: [AnyHashable: Any]) -> DeepLink? {
    guard let applicationId = userInfo["applicationId"] as? String else {
      return nil
    }
    return .applicationDetail(PlanningApplicationId(applicationId))
  }
}
