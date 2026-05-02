import SwiftUI
import TownCrierDomain

/// A button that is proactively gated by an entitlement check.
///
/// When the user's tier grants the required entitlement, tapping fires `action`.
/// When it does not, the button shows an ``UpgradeBadgeView`` overlay and tapping
/// fires `onUpgradeRequired` instead, which the parent uses to present the
/// subscription upsell sheet.
///
/// Usage:
/// ```swift
/// GatedButton(
///     label: "Status Change Alerts",
///     entitlement: .statusChangeAlerts,
///     featureGate: viewModel.featureGate,
///     action: { viewModel.enableAlerts() },
///     onUpgradeRequired: { viewModel.showUpgradeSheet() }
/// )
/// ```
public struct GatedButton: View {
  private let label: String
  private let entitlement: Entitlement
  private let featureGate: FeatureGate
  private let action: () -> Void
  private let onUpgradeRequired: () -> Void

  /// Whether the button performs its action (i.e. the user's tier grants the entitlement).
  public var isEnabled: Bool {
    featureGate.hasEntitlement(entitlement)
  }

  public init(
    label: String,
    entitlement: Entitlement,
    featureGate: FeatureGate,
    action: @escaping () -> Void,
    onUpgradeRequired: @escaping () -> Void
  ) {
    self.label = label
    self.entitlement = entitlement
    self.featureGate = featureGate
    self.action = action
    self.onUpgradeRequired = onUpgradeRequired
  }

  public var body: some View {
    Button {
      if isEnabled {
        action()
      } else {
        onUpgradeRequired()
      }
    } label: {
      HStack(spacing: TCSpacing.small) {
        Text(label)
          .font(TCTypography.body)
          .foregroundStyle(isEnabled ? Color.tcTextPrimary : Color.tcTextTertiary)
        if !isEnabled {
          UpgradeBadgeView()
        }
      }
    }
    .frame(minHeight: 44)
  }
}
