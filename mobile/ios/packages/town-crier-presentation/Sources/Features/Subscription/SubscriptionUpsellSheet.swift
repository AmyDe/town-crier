import SwiftUI
import TownCrierDomain

/// A reusable sheet presented when a user attempts an action above their subscription tier.
///
/// Presented via `.sheet(item:)` binding on a `@Published var entitlementGate: Entitlement?`
/// property on ViewModels that need gating.
public struct SubscriptionUpsellSheet: View {
  private let entitlement: Entitlement
  private let onViewPlans: () -> Void
  private let onDismiss: () -> Void

  public init(
    entitlement: Entitlement,
    onViewPlans: @escaping () -> Void,
    onDismiss: @escaping () -> Void
  ) {
    self.entitlement = entitlement
    self.onViewPlans = onViewPlans
    self.onDismiss = onDismiss
  }

  public var body: some View {
    VStack(spacing: TCSpacing.large) {
      Spacer()
        .frame(height: TCSpacing.medium)

      Image(systemName: "lock.fill")
        .font(TCTypography.displayLarge)
        .foregroundStyle(Color.tcAmber)

      Text("Upgrade to use this")
        .font(TCTypography.displaySmall)
        .foregroundStyle(Color.tcTextPrimary)

      Text(entitlement.displayName)
        .font(TCTypography.headline)
        .foregroundStyle(Color.tcAmber)

      Text(entitlement.featureDescription)
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

  func simulateViewPlansTap() {
    onViewPlans()
  }

  func simulateDismissTap() {
    onDismiss()
  }
}
