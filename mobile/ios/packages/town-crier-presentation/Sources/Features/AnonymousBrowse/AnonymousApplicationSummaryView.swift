import SwiftUI
import TownCrierDomain

/// A reduced-feature bottom sheet showing a preview of a tapped pin on the
/// anonymous map (GH#868 Phase 3). Unlike the authenticated
/// `ApplicationSummarySheet`, there is no save affordance — saving requires
/// an account — but "View full details" presents the full detail screen with
/// no sign-up gate (GH#879 Phase 2): the anonymous map already holds the full
/// `PlanningApplication`, so no network call is needed either.
struct AnonymousApplicationSummaryView: View {
  let application: PlanningApplication
  let onViewFullDetails: () -> Void

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
        onViewFullDetails()
      } label: {
        HStack {
          Image(systemName: "doc.text.magnifyingglass")
          Text("View full details")
        }
      }
      .accessibilityLabel("View full details")
    }
    .padding(TCSpacing.medium)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(Color.tcSurfaceElevated)
    .presentationDetents([.medium, .fraction(0.3)])
    .presentationDragIndicator(.visible)
  }
}
