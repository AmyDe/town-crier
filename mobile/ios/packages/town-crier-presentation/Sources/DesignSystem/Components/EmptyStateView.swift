import SwiftUI

/// Reusable empty state component following the design language.
/// Centered icon, title, description, and optional CTA button.
public struct EmptyStateView: View {
  private let icon: String
  private let title: String
  private let description: String
  private let actionLabel: String?
  private let action: (() -> Void)?

  public init(
    icon: String,
    title: String,
    description: String,
    actionLabel: String? = nil,
    action: (() -> Void)? = nil
  ) {
    self.icon = icon
    self.title = title
    self.description = description
    self.actionLabel = actionLabel
    self.action = action
  }

  public var body: some View {
    VStack(spacing: TCSpacing.medium) {
      Image(systemName: icon)
        .font(.system(.largeTitle))
        .foregroundStyle(Color.tcTextTertiary)

      Text(title)
        .font(TCTypography.headline)
        .foregroundStyle(Color.tcTextPrimary)

      Text(description)
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        .multilineTextAlignment(.center)

      if let actionLabel, let action {
        PrimaryButton(actionLabel, action: action)
          .padding(.horizontal, TCSpacing.medium)
      }
    }
    .padding(TCSpacing.extraLarge)
  }
}
