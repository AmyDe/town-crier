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
      noticeCard

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

  // MARK: - Filed-notice card (GH#857/#896)

  private var noticeCard: some View {
    VStack(alignment: .leading, spacing: TCSpacing.small) {
      // Mono document-header strip: reference leading, received date
      // trailing — mirrors `ApplicationListRow`'s mono metadata line.
      HStack(alignment: .top) {
        Text(application.reference.value)
          .font(TCTypography.monoEmphasis)
        Spacer()
        Text("Received \(application.receivedDate.formatted(date: .abbreviated, time: .omitted))")
          .font(TCTypography.mono)
          .multilineTextAlignment(.trailing)
      }
      .foregroundStyle(Color.tcTextSecondary)

      // Stamp status (GH#857) — the same `StatusBadgeView` the R4 restyle
      // built, never a forked copy.
      StatusBadgeView(status: application.status)

      Text(application.description)
        .font(TCTypography.headline)
        .foregroundStyle(Color.tcTextPrimary)

      Label(application.address, systemImage: "mappin.and.ellipse")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
    }
    .padding(TCSpacing.medium)
    .noticeCardStyle()
  }
}
