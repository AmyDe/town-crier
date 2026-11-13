import SwiftUI
import TownCrierDomain

extension View {
  /// Presents a ``SubscriptionUpsellSheet`` when the bound entitlement is non-nil.
  /// Both "Not now" and "View Plans" clear the binding; "View Plans" also fires
  /// `onViewPlans`, after the binding clears so the sheet dismisses before navigation.
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
      .selfSizingSheet()
    }
  }
}
