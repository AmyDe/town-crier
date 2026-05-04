import TownCrierDomain

/// Extracts deep-link destinations and renders user-facing copy from push
/// notification payloads delivered by APNs.
///
/// Decision-update events arrive with a raw PlanIt `app_state` (e.g.
/// `Permitted`) and must be expanded into the UK vocabulary residents
/// recognise (`Approved`, `Approved with conditions`, `Refused`,
/// `Refusal appealed`) at render time. ``DecisionVocabulary`` owns the
/// mapping; this parser owns the payload schema and sentence shape.
public enum NotificationPayloadParser {
  public static func parseDeepLink(from userInfo: [AnyHashable: Any]) -> DeepLink? {
    // The APNs payload uses `applicationRef` per docs/specs/apns-push-sender.md
    // and api/src/town-crier.infrastructure/Notifications/ApnsAlertPayload.cs.
    // Digest pushes have no applicationRef — those return nil here and the
    // delegate must still complete on MainActor (see NotificationDelegate).
    guard let applicationRef = userInfo["applicationRef"] as? String else {
      return nil
    }
    return .applicationDetail(PlanningApplicationId(applicationRef))
  }

  /// Renders the user-facing body string for a decision-update push payload.
  ///
  /// Returns `nil` for any payload that is not a recognised
  /// `DecisionUpdate` event with a known PlanIt decision state — callers
  /// fall back to whatever body APNs already delivered (i.e. the unmodified
  /// `aps.alert.body`).
  ///
  /// Sentence shape: `"Application <name> was <UK term>"`, e.g.
  /// `"Application 12345 was Approved with conditions"`.
  ///
  /// - Parameter userInfo: The raw `userInfo` dictionary from a
  ///   `UNNotification` request.
  /// - Returns: The UK-vocabulary body string, or `nil` if the payload is
  ///   not a renderable decision update.
  public static func renderBody(from userInfo: [AnyHashable: Any]) -> String? {
    guard let eventType = userInfo["eventType"] as? String,
      eventType == "DecisionUpdate"
    else {
      return nil
    }
    guard let applicationName = userInfo["applicationName"] as? String,
      !applicationName.isEmpty
    else {
      return nil
    }
    let rawDecision = userInfo["decision"] as? String
    guard let label = DecisionVocabulary.displayLabel(forPlanItAppState: rawDecision) else {
      return nil
    }
    return "Application \(applicationName) was \(label)"
  }
}
