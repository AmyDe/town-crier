import SwiftUI
import TownCrierDomain

/// Filterable list of planning applications within a watch zone.
public struct ApplicationListView: View {
  @StateObject private var viewModel: ApplicationListViewModel

  public init(viewModel: ApplicationListViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    VStack(spacing: 0) {
      if viewModel.showZonePicker {
        zonePickerHeader
      }

      if viewModel.canFilter || viewModel.canSave {
        filterHeader
      }

      listBody
    }
    .background(Color.tcBackground)
    .navigationTitle("Applications")
    #if os(iOS)
      .navigationBarTitleDisplayMode(.large)
    #endif
    .task {
      await viewModel.loadApplications()
    }
    .refreshable {
      await viewModel.loadApplications()
    }
  }

  // MARK: - List Body

  private var listBody: some View {
    ZStack {
      Color.tcBackground.ignoresSafeArea()

      if viewModel.isLoading && viewModel.filteredApplications.isEmpty {
        ListSkeletonView()
      } else if let error = viewModel.error {
        ErrorStateView(error: error) {
          await viewModel.loadApplications()
        }
      } else if viewModel.isEmpty {
        if viewModel.isSavedFilterActive {
          EmptyStateView(
            icon: "bookmark",
            title: "No Saved Applications",
            description:
              "No saved applications. Tap the bookmark icon on any application to save it."
          )
        } else {
          EmptyStateView(
            icon: "doc.text.magnifyingglass",
            title: "No Applications",
            description: "No planning applications found in your watch zone yet."
          )
        }
      } else {
        List {
          ForEach(viewModel.filteredApplications) { application in
            ApplicationListRow(application: application)
              .listRowBackground(Color.tcSurface)
              .contentShape(Rectangle())
              .onTapGesture {
                viewModel.selectApplication(application.id)
              }
          }
        }
        .listStyle(.plain)
      }
    }
  }

  // MARK: - Zone Picker

  private var zonePickerHeader: some View {
    ZonePickerView(
      zones: viewModel.zones,
      selectedZoneId: viewModel.selectedZone?.id
    ) { zone in
      Task {
        await viewModel.selectZone(zone)
      }
    }
    .background(Color.tcBackground)
  }

  // MARK: - Filter Section

  private var filterHeader: some View {
    ScrollView(.horizontal, showsIndicators: false) {
      HStack(spacing: TCSpacing.small) {
        if viewModel.canSave {
          savedFilterChip
        }
        if viewModel.canFilter {
          filterChip(label: "All", status: nil)
          filterChip(label: "Pending", status: .undecided)
          filterChip(label: "Approved", status: .approved)
          filterChip(label: "Refused", status: .refused)
          filterChip(label: "Withdrawn", status: .withdrawn)
          filterChip(label: "Appealed", status: .appealed)
        }
      }
      .padding(.horizontal, TCSpacing.medium)
      .padding(.vertical, TCSpacing.small)
    }
    .background(Color.tcBackground)
  }

  private var savedFilterChip: some View {
    Button {
      if viewModel.isSavedFilterActive {
        viewModel.deactivateSavedFilter()
      } else {
        Task { await viewModel.activateSavedFilter() }
      }
    } label: {
      Label("Saved", systemImage: "bookmark.fill")
        .font(TCTypography.captionEmphasis)
        .foregroundStyle(
          viewModel.isSavedFilterActive ? Color.tcTextOnAccent : Color.tcTextPrimary
        )
        .padding(.horizontal, TCSpacing.small)
        .padding(.vertical, TCSpacing.extraSmall)
        .background(viewModel.isSavedFilterActive ? Color.tcAmber : Color.tcSurface)
        .clipShape(Capsule())
        .overlay(
          Capsule()
            .stroke(Color.tcBorder, lineWidth: viewModel.isSavedFilterActive ? 0 : 1)
        )
    }
    .buttonStyle(.plain)
  }

  private func filterChip(label: String, status: ApplicationStatus?) -> some View {
    let isSelected = viewModel.selectedStatusFilter == status
      && !viewModel.isSavedFilterActive
    return Button {
      viewModel.selectedStatusFilter = status
    } label: {
      Text(label)
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
    }
    .buttonStyle(.plain)
  }

}
