import SwiftUI
import TownCrierDomain

/// The dedicated Saved tab — a flat, cross-zone list of the user's bookmarked
/// applications, with a status filter pill row that's free for all tiers. Tap
/// rows to open `ApplicationDetailView` (sheet wired by `TownCrierApp`).
public struct SavedApplicationListView: View {
  @StateObject private var viewModel: SavedApplicationListViewModel

  public init(viewModel: SavedApplicationListViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    List {
      filterRow
      contentRows
    }
    .listStyle(.plain)
    .scrollContentBackground(.hidden)
    .background(Color.tcBackground)
    .navigationTitle("Saved")
    #if os(iOS)
      .navigationBarTitleDisplayMode(.large)
    #endif
    .task {
      await viewModel.loadAll()
    }
    .refreshable {
      await viewModel.loadAll()
    }
  }

  // MARK: - Filter Row

  @ViewBuilder
  private var filterRow: some View {
    if !viewModel.applications.isEmpty {
      ScrollView(.horizontal, showsIndicators: false) {
        HStack(spacing: TCSpacing.small) {
          filterChip(label: "All", status: nil)
          filterChip(label: "Pending", status: .undecided)
          filterChip(label: "Granted", status: .permitted)
          filterChip(label: "Granted with conditions", status: .conditions)
          filterChip(label: "Refused", status: .rejected)
          filterChip(label: "Withdrawn", status: .withdrawn)
          filterChip(label: "Appealed", status: .appealed)
        }
        .padding(.horizontal, TCSpacing.medium)
        .padding(.vertical, TCSpacing.small)
      }
      .listRowSeparator(.hidden)
      .listRowInsets(EdgeInsets())
      .listRowBackground(Color.tcBackground)
    }
  }

  private func filterChip(label: String, status: ApplicationStatus?) -> some View {
    let isSelected = viewModel.selectedStatusFilter == status
    return Text(label)
      .font(TCTypography.captionEmphasis)
      .foregroundStyle(isSelected ? Color.tcTextOnAccent : Color.tcTextPrimary)
      .padding(.horizontal, TCSpacing.small)
      .padding(.vertical, TCSpacing.extraSmall)
      .background(isSelected ? Color.tcAmber : Color.tcSurface)
      .clipShape(Capsule())
      .overlay(
        Capsule()
          .stroke(Color.tcBorder, lineWidth: isSelected ? 0 : 1)
      )
      .contentShape(Capsule())
      .onTapGesture {
        viewModel.selectedStatusFilter = status
      }
  }

  // MARK: - Content Rows

  @ViewBuilder
  private var contentRows: some View {
    if viewModel.isLoading && viewModel.applications.isEmpty {
      ListSkeletonView()
        .listRowSeparator(.hidden)
        .listRowInsets(EdgeInsets())
        .listRowBackground(Color.tcBackground)
    } else if let error = viewModel.error {
      ErrorStateView(error: error) {
        await viewModel.loadAll()
      }
      .frame(maxWidth: .infinity)
      .listRowSeparator(.hidden)
      .listRowInsets(EdgeInsets())
      .listRowBackground(Color.tcBackground)
    } else if viewModel.isEmpty {
      emptyStateRow
        .listRowSeparator(.hidden)
        .listRowInsets(EdgeInsets())
        .listRowBackground(Color.tcBackground)
    } else {
      ForEach(viewModel.filteredApplications) { application in
        ApplicationListRow(application: application)
          .listRowBackground(Color.tcSurface)
          .contentShape(Rectangle())
          .onTapGesture {
            viewModel.selectApplication(application.id)
          }
      }
    }
  }

  @ViewBuilder
  private var emptyStateRow: some View {
    if viewModel.selectedStatusFilter == nil {
      EmptyStateView(
        icon: "bookmark",
        title: "No Saved Applications",
        description:
          "Bookmark applications you want to track. Tap the bookmark icon on any application detail."
      )
      .frame(maxWidth: .infinity)
    } else {
      EmptyStateView(
        icon: "line.3.horizontal.decrease.circle",
        title: "No Matches",
        description: "No saved applications match this filter."
      )
      .frame(maxWidth: .infinity)
    }
  }
}
