import SwiftUI
import TownCrierDomain

/// A toggle that is proactively disabled when the user's tier lacks the required entitlement.
///
/// When the entitlement is not granted, the toggle is disabled and an ``UpgradeBadgeView``
/// is shown alongside. Tapping the disabled row triggers `onUpgradeRequired`, which the
/// parent can use to present the subscription upsell sheet.
///
/// Usage:
/// ```swift
/// GatedToggle(
///     label: "Status Changes",
///     isOn: $statusChangesEnabled,
///     entitlement: .statusChangeAlerts,
///     featureGate: viewModel.featureGate
/// ) {
///     viewModel.showUpgradeSheet()
/// }
/// ```
public struct GatedToggle: View {
  private let label: String
  @Binding private var isOn: Bool
  private let entitlement: Entitlement
  private let featureGate: FeatureGate
  private let onUpgradeRequired: () -> Void

  /// Whether the toggle is interactive (i.e. the user's tier grants the entitlement).
  public var isEnabled: Bool {
    featureGate.hasEntitlement(entitlement)
  }

  public init(
    label: String,
    isOn: Binding<Bool>,
    entitlement: Entitlement,
    featureGate: FeatureGate,
    onUpgradeRequired: @escaping () -> Void
  ) {
    self.label = label
    self._isOn = isOn
    self.entitlement = entitlement
    self.featureGate = featureGate
    self.onUpgradeRequired = onUpgradeRequired
  }

  public var body: some View {
    if isEnabled {
      Toggle(label, isOn: $isOn)
        .tint(Color.tcAmber)
    } else {
      Button {
        onUpgradeRequired()
      } label: {
        HStack {
          Text(label)
            .foregroundStyle(Color.tcTextTertiary)
          Spacer()
          UpgradeBadgeView()
        }
      }
      .frame(minHeight: 44)
    }
  }
}
