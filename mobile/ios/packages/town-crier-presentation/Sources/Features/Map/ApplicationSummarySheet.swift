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
            .font(TCTypography.body)
          }
          .accessibilityLabel(viewModel.isSelectedApplicationSaved ? "Unsave" : "Save")
        }
        // Mono metadata (GH#857): the planning reference, previously plain
        // body text.
        Text(application.reference.value)
          .font(TCTypography.mono)
          .foregroundStyle(Color.tcTextSecondary)
      }

      Text(application.description)
        .font(TCTypography.headline)
        .foregroundStyle(Color.tcTextPrimary)

      Label(application.address, systemImage: "mappin.and.ellipse")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)

      Text("Received \(application.receivedDate.formatted(date: .abbreviated, time: .omitted))")
        .font(TCTypography.mono)
        .foregroundStyle(Color.tcTextTertiary)

      PrimaryButton {
        viewModel.requestFullDetail()
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
    .task {
      await viewModel.loadSavedStateForSelectedApplication()
    }
  }

}
