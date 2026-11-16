import SwiftUI
import TownCrierDomain

/// The anonymous (pre-signup) Applications tab: a single page of planning
/// applications reusing ``ApplicationListRow``, with no sort or filter chips.
/// Once a ``DeviceLocalZone`` exists, a zone chip row with a trailing
/// "Add area" chip always renders above the list.
public struct AnonymousApplicationListView: View {
  @StateObject private var viewModel: AnonymousApplicationListViewModel

  public init(viewModel: AnonymousApplicationListViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    List {
      mastheadRow
      zonePickerRow
      contentRows
    }
    .listStyle(.plain)
    .scrollContentBackground(.hidden)
    .background(Color.tcBackground)
    .navigationTitle("Applications")
    .mastheadNavigationBar()
    .task {
      await viewModel.loadApplications()
    }
    .refreshable {
      await viewModel.loadApplications()
    }
    .sheet(isPresented: $viewModel.isSignUpCTAPresented) {
      DeviceLocalZoneSignUpCTAView(
        onCreateAccount: { viewModel.confirmSignUp() },
        onSignIn: { viewModel.confirmSignUp() },
        onDismiss: { viewModel.dismissSignUpCTA() }
      )
      .selfSizingSheet()
    }
  }

  // MARK: - Masthead

  private var mastheadRow: some View {
    MastheadView(title: "Applications")
      .padding(.horizontal, TCSpacing.medium)
      .padding(.top, TCSpacing.small)
      .padding(.bottom, TCSpacing.extraSmall)
      .listRowSeparator(.hidden)
      .listRowInsets(EdgeInsets())
      .listRowBackground(Color.tcBackground)
  }

  // MARK: - Zone picker

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

          // GH#888: the on-device cap is one zone — adding another always
          // routes to the sign-up CTA rather than a device-local editor.
          CapsuleChipView(
            label: "+ Add area",
            isSelected: false
          ) {
            viewModel.requestAddArea()
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
        // Filed-notice card (GH#857/#896): `ApplicationListRow` already
        // applies `NoticeCardStyle` internally — `cardRowInsets()` clears
        // the List's own row padding/background/separator so the card's
        // rounded corners and border actually show, mirroring the authed
        // `ApplicationListView`.
        ApplicationListRow(application: application)
          .cardRowInsets()
          .contentShape(Rectangle())
          .onTapGesture {
            viewModel.selectApplication(application)
          }
      }
    }
  }
}
