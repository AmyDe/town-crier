import SwiftUI
import TownCrierDomain

/// The anonymous (pre-signup) Applications tab (GH#879 Phase 3): a single
/// nearest-first page of planning applications, reusing the same
/// ``ApplicationListRow`` the authenticated Applications tab uses. No
/// sort/filter chips (pre-resolved: v1 is nearest-first only).
///
/// GH#879 Phase 4: when more than one device-local zone exists, a zone
/// picker chip row appears above the list — mirroring the authed
/// `ApplicationListView`'s zone chips (`ApplicationListView.swift:120-140`)
/// but over ``DeviceLocalZone``. Switching the active zone re-fetches this
/// list and re-centres the Map tab.
public struct AnonymousApplicationListView: View {
  @StateObject private var viewModel: AnonymousApplicationListViewModel

  public init(viewModel: AnonymousApplicationListViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    List {
      zonePickerRow
      contentRows
    }
    .listStyle(.plain)
    .scrollContentBackground(.hidden)
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

  // MARK: - Zone picker (GH#879 Phase 4)

  @ViewBuilder
  private var zonePickerRow: some View {
    if viewModel.showZonePicker {
      ScrollView(.horizontal, showsIndicators: false) {
        HStack(spacing: TCSpacing.small) {
          ForEach(viewModel.zones) { zone in
            CapsuleChipView(
              label: zone.name,
              isSelected: zone.id == viewModel.selectedZone?.id
            ) {
              Task {
                await viewModel.selectZone(zone)
              }
            }
          }
        }
        .padding(.horizontal, TCSpacing.medium)
        .padding(.vertical, TCSpacing.small)
      }
      .listRowSeparator(.hidden)
      .listRowInsets(EdgeInsets())
      .listRowBackground(Color.tcBackground)
    }
  }

  @ViewBuilder
  private var contentRows: some View {
    if viewModel.isLoading && viewModel.applications.isEmpty {
      ListSkeletonView()
        .listRowSeparator(.hidden)
        .listRowInsets(EdgeInsets())
        .listRowBackground(Color.tcBackground)
    } else if let error = viewModel.error {
      ErrorStateView(error: error) {
        await viewModel.loadApplications()
      }
      .frame(maxWidth: .infinity)
      .listRowSeparator(.hidden)
      .listRowInsets(EdgeInsets())
      .listRowBackground(Color.tcBackground)
    } else if viewModel.isEmpty {
      EmptyStateView(
        icon: "doc.text.magnifyingglass",
        title: "No Applications Nearby",
        description: "No planning applications found within your chosen area yet."
      )
      .frame(maxWidth: .infinity)
      .listRowSeparator(.hidden)
      .listRowInsets(EdgeInsets())
      .listRowBackground(Color.tcBackground)
    } else {
      ForEach(viewModel.applications) { application in
        ApplicationListRow(application: application)
          .listRowBackground(Color.tcSurface)
          .contentShape(Rectangle())
          .onTapGesture {
            viewModel.selectApplication(application)
          }
      }
    }
  }
}
