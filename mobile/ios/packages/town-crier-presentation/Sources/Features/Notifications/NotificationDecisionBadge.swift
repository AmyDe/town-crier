import SwiftUI
import TownCrierDomain

/// Compact UK-vocabulary decision chip rendered inside a notification row when
/// the item represents a decision update with a recognised PlanIt `app_state`.
///
/// Badge visibility is gated by ``displayLabel(for:)`` so a notification can
/// silently degrade for older event types or unknown decision vocabulary
/// rather than show a misleading status. The chip itself uses the design
/// language's status palette: green for granted, amber for granted-with-
/// conditions, red for refused, purple for appeals.
struct NotificationDecisionBadge: View {
  let item: NotificationItem

  var body: some View {
    if let label = Self.displayLabel(for: item) {
      let color = Self.color(for: item.decision)
      HStack(spacing: TCSpacing.extraSmall) {
        Image(systemName: Self.icon(for: item.decision))
        Text(label)
      }
      .font(TCTypography.captionEmphasis)
      .foregroundStyle(color)
      .padding(.horizontal, TCSpacing.small)
      .padding(.vertical, TCSpacing.extraSmall)
      .background(color.opacity(0.15))
      .clipShape(Capsule())
      .accessibilityElement(children: .combine)
      .accessibilityLabel("Decision: \(label)")
    }
  }

  // MARK: - Public helpers

  /// Returns the UK display label for `item` when it should render a decision
  /// badge, or `nil` to suppress the badge entirely.
  ///
  /// Suppression cases:
  /// * `eventType` is anything other than `"DecisionUpdate"`.
  /// * `decision` is `nil`, empty, or not in the recognised PlanIt vocabulary.
  static func displayLabel(for item: NotificationItem) -> String? {
    guard item.eventType == "DecisionUpdate" else { return nil }
    return DecisionVocabulary.displayLabel(forPlanItAppState: item.decision)
  }

  // MARK: - Styling

  private static func color(for decision: String?) -> Color {
    switch decision?.lowercased() {
    case "permitted":
      .tcStatusPermitted
    case "conditions":
      .tcStatusConditions
    case "rejected":
      .tcStatusRejected
    case "appealed":
      .tcStatusAppealed
    default:
      .tcTextTertiary
    }
  }

  private static func icon(for decision: String?) -> String {
    switch decision?.lowercased() {
    case "permitted":
      "checkmark.circle"
    case "conditions":
      "checkmark.circle.badge.questionmark"
    case "rejected":
      "xmark.circle"
    case "appealed":
      "exclamationmark.triangle"
    default:
      "questionmark.circle"
    }
  }
}
