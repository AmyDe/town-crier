import Foundation

/// Maps PlanIt's wire vocabulary (`Permitted`, `Conditions`, `Rejected`,
/// `Appealed`) to the user-facing UK planning terms residents recognise
/// (`Approved`, `Approved with conditions`, `Refused`, `Refusal appealed`).
///
/// Mirrors the API-side `UkPlanningVocabulary` helper so push payloads,
/// in-app notification rows, and any future shared rendering surface stay
/// in sync. Centralising the mapping closes the prior gap where decision
/// states dropped to `.unknown` on iOS.
///
/// See `docs/specs/decision-state-vocabulary.md`.
public enum DecisionVocabulary {

  /// Returns the UK display string for a PlanIt `app_state` value, or `nil`
  /// when the input is not one of the four decision states this helper
  /// covers. Matching is case-insensitive to tolerate upstream casing drift.
  ///
  /// - Parameter planItAppState: The raw PlanIt `app_state` string carried
  ///   on a notification payload (e.g. `"Permitted"`).
  /// - Returns: The UK display label, or `nil` when the input is not a
  ///   recognised decision state.
  public static func displayLabel(forPlanItAppState planItAppState: String?) -> String? {
    guard let raw = planItAppState?.trimmingCharacters(in: .whitespaces), !raw.isEmpty else {
      return nil
    }
    switch raw.lowercased() {
    case "permitted":
      return "Approved"
    case "conditions":
      return "Approved with conditions"
    case "rejected":
      return "Refused"
    case "appealed":
      return "Refusal appealed"
    default:
      return nil
    }
  }
}
