import Foundation
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

  /// Extracts the notification's `createdAt` instant from an APNs userInfo
  /// dictionary, used to advance the read-state watermark on push-tap (spec
  /// `docs/specs/notifications-unread-watermark.md#ios-badge-foreground-push`).
  ///
  /// The server emits ISO-8601 strings (with or without fractional seconds).
  /// The value is decoded explicitly here because `userInfo` is a plist-style
  /// `[AnyHashable: Any]`, not JSON, so `JSONDecoder` strategies do not apply.
  ///
  /// Returns `nil` when the key is missing, the value is not a string, or the
  /// string cannot be parsed — older API builds and digest pushes do not
  /// carry `createdAt`, and the push-tap deep-link path must continue to work
  /// in those cases (advance is fire-and-forget).
  public static func parseCreatedAt(from userInfo: [AnyHashable: Any]) -> Date? {
    guard let raw = userInfo["createdAt"] as? String else {
      return nil
    }
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime]
    if let date = formatter.date(from: raw) {
      return date
    }
    formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
    return formatter.date(from: raw)
  }
}
