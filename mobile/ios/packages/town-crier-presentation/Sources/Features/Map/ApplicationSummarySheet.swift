import SwiftUI
import TownCrierDomain

/// A bottom sheet showing a summary of a selected planning application.
struct ApplicationSummarySheet: View {
  let application: PlanningApplication
  @ObservedObject var viewModel: MapViewModel

  var body: some View {
    VStack(alignment: .leading, spacing: TCSpacing.medium) {
      HStack {
        StatusBadgeView(status: application.status)
        Spacer()
        if viewModel.canSave {
          Button {
            Task { await viewModel.toggleSaveSelectedApplication() }
          } label: {
            Image(
              systemName: viewModel.isSelectedApplicationSaved ? "bookmark.fill" : "bookmark"
            )
            .foregroundStyle(
              viewModel.isSelectedApplicationSaved ? Color.tcAmber : Color.tcTextSecondary
            )
            .font(.system(.body))
          }
          .accessibilityLabel(viewModel.isSelectedApplicationSaved ? "Unsave" : "Save")
        }
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
    }
    .padding(TCSpacing.medium)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(Color.tcSurfaceElevated)
    .presentationDetents([.medium, .fraction(0.3)])
    .presentationDragIndicator(.visible)
    .task {
      await viewModel.loadSavedStateForSelectedApplication()
    }
  }

}
