import SwiftUI
import TownCrierDomain

/// A reduced-feature bottom sheet showing a preview of a tapped pin on the
/// anonymous map (GH#868 Phase 3). Unlike the authenticated
/// `ApplicationSummarySheet`, there is no save affordance and no "View full
/// details" — any deeper look routes to sign-up instead, since full detail and
/// saving both require an account.
struct AnonymousApplicationSummaryView: View {
  let application: PlanningApplication
  let onSignUp: () -> Void

  var body: some View {
    VStack(alignment: .leading, spacing: TCSpacing.medium) {
      HStack {
        StatusBadgeView(status: application.status)
        Spacer()
        Text(application.reference.value)
          .font(.system(.caption))
          .foregroundStyle(Color.tcTextSecondary)
      }

      Text(application.description)
        .font(.system(.headline, weight: .semibold))
        .foregroundStyle(Color.tcTextPrimary)

      Label(application.address, systemImage: "mappin.and.ellipse")
        .font(.system(.body))
        .foregroundStyle(Color.tcTextSecondary)

      Text("Received \(application.receivedDate.formatted(date: .abbreviated, time: .omitted))")
        .font(.system(.caption))
        .foregroundStyle(Color.tcTextTertiary)

      PrimaryButton {
        onSignUp()
      } label: {
        HStack {
          Image(systemName: "bell.badge")
          Text("Create free account for full details")
        }
      }
      .accessibilityLabel("Create free account for full details")
    }
    .padding(TCSpacing.medium)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(Color.tcSurfaceElevated)
    .presentationDetents([.medium, .fraction(0.3)])
    .presentationDragIndicator(.visible)
  }
}
