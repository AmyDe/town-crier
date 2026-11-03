import SwiftUI

/// A sheet presented when a free-tier user attempts to add a watch zone beyond
/// their tier's limit.
///
/// Shows a zone-specific value proposition with a CTA to view subscription plans
/// and a secondary dismiss action.
///
/// Presented via `.sheet(isPresented:)` binding on
/// ``WatchZoneListViewModel/isUpgradePromptPresented``.
public struct WatchZoneUpsellView: View {
  private let valueProposition: String
  private let onViewPlans: () -> Void
  private let onDismiss: () -> Void

  public init(
    valueProposition: String,
    onViewPlans: @escaping () -> Void,
    onDismiss: @escaping () -> Void
  ) {
    self.valueProposition = valueProposition
    self.onViewPlans = onViewPlans
    self.onDismiss = onDismiss
  }

  public var body: some View {
    VStack(spacing: TCSpacing.large) {
      Spacer()
        .frame(height: TCSpacing.medium)

      Image(systemName: "mappin.and.ellipse")
        .font(TCTypography.displayLarge)
        .foregroundStyle(Color.tcAmber)

      Text("Add more watch zones")
        .font(TCTypography.displaySmall)
        .foregroundStyle(Color.tcTextPrimary)

      Text(valueProposition)
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        .multilineTextAlignment(.center)
        .padding(.horizontal, TCSpacing.medium)

      Spacer()
        .frame(height: TCSpacing.small)

      PrimaryButton("View Plans", action: onViewPlans)
        .padding(.horizontal, TCSpacing.medium)

      Button(action: onDismiss) {
        Text("Not now")
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextSecondary)
      }
      .frame(minHeight: 44)

      Spacer()
        .frame(height: TCSpacing.medium)
    }
    .padding(.horizontal, TCSpacing.medium)
    .background(Color.tcSurfaceElevated)
  }

  // MARK: - Test Helpers

  /// Simulates tapping "View Plans" for unit testing.
  func simulateViewPlansTap() {
    onViewPlans()
  }

  /// Simulates tapping "Not now" for unit testing.
  func simulateDismissTap() {
    onDismiss()
  }
}
