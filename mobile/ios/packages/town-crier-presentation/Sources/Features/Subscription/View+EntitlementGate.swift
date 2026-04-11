import SwiftUI
import TownCrierDomain

extension View {
  /// Presents a ``SubscriptionUpsellSheet`` when the bound entitlement is non-nil.
  ///
  /// Usage:
  /// ```swift
  /// .entitlementGateSheet(entitlement: $viewModel.entitlementGate) {
  ///     coordinator.showSubscription()
  /// }
  /// ```
  ///
  /// The sheet automatically dismisses when the user taps "Not now", clearing
  /// the binding. Tapping "View Plans" fires `onViewPlans` and also clears
  /// the binding so the sheet dismisses before navigation begins.
  public func entitlementGateSheet(
    entitlement: Binding<Entitlement?>,
    onViewPlans: @escaping () -> Void
  ) -> some View {
    sheet(item: entitlement) { gatedEntitlement in
      SubscriptionUpsellSheet(
        entitlement: gatedEntitlement,
        onViewPlans: {
          entitlement.wrappedValue = nil
          onViewPlans()
        },
        onDismiss: {
          entitlement.wrappedValue = nil
        }
      )
      .presentationDetents([.medium])
      .presentationDragIndicator(.visible)
    }
  }
}
