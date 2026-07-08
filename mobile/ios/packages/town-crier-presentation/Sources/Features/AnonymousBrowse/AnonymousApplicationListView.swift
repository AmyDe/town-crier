import SwiftUI
import TownCrierDomain

/// The anonymous (pre-signup) Applications tab (GH#879 Phase 3): a single
/// nearest-first page of planning applications, reusing the same
/// ``ApplicationListRow`` the authenticated Applications tab uses. No
/// sort/filter chips (pre-resolved: v1 is nearest-first only) and no zone
/// picker — the anonymous session has exactly one area, seeded from the
/// postcode entered before this screen.
public struct AnonymousApplicationListView: View {
  @StateObject private var viewModel: AnonymousApplicationListViewModel

  public init(viewModel: AnonymousApplicationListViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    List {
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
