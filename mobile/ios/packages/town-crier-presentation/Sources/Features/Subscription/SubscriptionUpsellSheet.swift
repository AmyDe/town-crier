import SwiftUI
import TownCrierDomain

/// A reusable sheet presented when a user attempts an action above their subscription tier.
///
/// Design (from spec 2.5):
/// - Title: "Upgrade to unlock"
/// - Body: describes the specific feature (parameterised by entitlement)
/// - CTA: "View Plans" -- navigates to existing `SubscriptionView`
/// - Secondary: "Not now" -- dismisses
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

            // Icon
            Image(systemName: "lock.fill")
                .font(.system(.largeTitle))
                .foregroundStyle(Color.tcAmber)

            // Title
            Text("Upgrade to unlock")
                .font(TCTypography.displaySmall)
                .foregroundStyle(Color.tcTextPrimary)

            // Feature name
            Text(entitlement.displayName)
                .font(TCTypography.headline)
                .foregroundStyle(Color.tcAmber)

            // Feature description
            Text(entitlement.featureDescription)
                .font(TCTypography.body)
                .foregroundStyle(Color.tcTextSecondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal, TCSpacing.medium)

            Spacer()
                .frame(height: TCSpacing.small)

            // Primary CTA
            PrimaryButton("View Plans", action: onViewPlans)
                .padding(.horizontal, TCSpacing.medium)

            // Secondary dismiss
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
