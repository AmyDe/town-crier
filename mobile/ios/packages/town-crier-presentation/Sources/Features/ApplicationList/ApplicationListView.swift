import SwiftUI
import TownCrierDomain

/// Filterable list of planning applications within a watch zone.
///
/// Uses a single `List` as the sole scroll container so that
/// `.navigationBarTitleDisplayMode(.large)` has one unambiguous
/// scroll view to track. Previous designs stacked horizontal
/// `ScrollView`s in a `VStack` above the `List`; the large-title
/// navigation bar hijacked the first one for its collapse
/// animation, corrupting its rendering (chips invisible but
/// still tappable).
///
/// The unread-watermark UI (tc-1nsa.8) layers on:
/// - an Unread filter chip (visible only when `hasUnread`) that mirrors the
///   web bead's single-select behaviour with the existing status chips,
/// - a sort menu in the toolbar with the four sort modes from the spec,
/// - a Mark-All-Read toolbar action (visible only when `hasUnread`).
public struct ApplicationListView: View {
  @StateObject private var viewModel: ApplicationListViewModel

  public init(viewModel: ApplicationListViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    List {
      chipRows
      contentRows
    }
    .listStyle(.plain)
    .scrollContentBackground(.hidden)
    .background(Color.tcBackground)
    .navigationTitle("Applications")
    #if os(iOS)
      .navigationBarTitleDisplayMode(.large)
    #endif
    .toolbar {
      sortToolbarItem
      markAllReadToolbarItem
    }
    .task {
      await viewModel.loadApplications()
    }
    .refreshable {
      await viewModel.loadApplications()
    }
  }

  // MARK: - Toolbar

  @ToolbarContentBuilder
  private var sortToolbarItem: some ToolbarContent {
    ToolbarItem(placement: sortToolbarPlacement) {
      Menu {
        Picker("Sort", selection: sortBinding) {
          ForEach(ApplicationsSort.allCases, id: \.self) { mode in
            Text(mode.displayLabel).tag(mode)
          }
        }
      } label: {
        Image(systemName: "arrow.up.arrow.down")
          .foregroundStyle(Color.tcTextPrimary)
      }
      .accessibilityLabel("Sort")
    }
  }

  @ToolbarContentBuilder
  private var markAllReadToolbarItem: some ToolbarContent {
    if viewModel.hasUnread {
      ToolbarItem(placement: sortToolbarPlacement) {
        Button("Mark all read") {
          Task { await viewModel.markAllRead() }
        }
        .foregroundStyle(Color.tcTextPrimary)
        .accessibilityLabel("Mark all read")
      }
    }
  }

  private var sortToolbarPlacement: ToolbarItemPlacement {
    #if os(iOS)
      return .topBarTrailing
    #else
      return .automatic
    #endif
  }

  /// Two-way binding into the ViewModel's `sort` property — kept as a
  /// computed property so the `Picker` re-renders correctly when the
  /// persisted choice is read on launch.
  private var sortBinding: Binding<ApplicationsSort> {
    Binding(
      get: { viewModel.sort },
      set: { viewModel.sort = $0 }
    )
  }

  // MARK: - Chip Rows

  @ViewBuilder
  private var chipRows: some View {
    if viewModel.showZonePicker {
      ScrollView(.horizontal, showsIndicators: false) {
        HStack(spacing: TCSpacing.small) {
          ForEach(viewModel.zones) { zone in
            zoneChip(zone: zone, isSelected: zone.id == viewModel.selectedZone?.id)
          }
        }
        .padding(.horizontal, TCSpacing.medium)
        .padding(.vertical, TCSpacing.small)
      }
      .listRowSeparator(.hidden)
      .listRowInsets(EdgeInsets())
      .listRowBackground(Color.tcBackground)
    }

    if !viewModel.applications.isEmpty {
      ScrollView(.horizontal, showsIndicators: false) {
        HStack(spacing: TCSpacing.small) {
          if viewModel.hasUnread {
            unreadChip
          }
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

  // MARK: - Content Rows

  @ViewBuilder
  private var contentRows: some View {
    if viewModel.isLoading && viewModel.filteredApplications.isEmpty {
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
        title: "No Applications",
        description: "No planning applications found in your watch zone yet."
      )
      .frame(maxWidth: .infinity)
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

  // MARK: - Zone Chip

  private func zoneChip(zone: WatchZone, isSelected: Bool) -> some View {
    Text(zone.name)
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
        Task {
          await viewModel.selectZone(zone)
        }
      }
  }

  // MARK: - Filter Chips

  private func filterChip(label: String, status: ApplicationStatus?) -> some View {
    let isSelected = !viewModel.unreadOnly && viewModel.selectedStatusFilter == status
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

  /// The Unread chip lives at the head of the chip group. Selecting it
  /// clears any active status filter and toggles `unreadOnly`. The label
  /// includes the watermark count so it doubles as the unread badge.
  private var unreadChip: some View {
    Text("Unread (\(viewModel.unreadCount))")
      .font(TCTypography.captionEmphasis)
      .foregroundStyle(viewModel.unreadOnly ? Color.tcTextOnAccent : Color.tcTextPrimary)
      .padding(.horizontal, TCSpacing.small)
      .padding(.vertical, TCSpacing.extraSmall)
      .background(viewModel.unreadOnly ? Color.tcAmber : Color.tcSurface)
      .clipShape(Capsule())
      .overlay(
        Capsule()
          .stroke(Color.tcBorder, lineWidth: viewModel.unreadOnly ? 0 : 1)
      )
      .contentShape(Capsule())
      .onTapGesture {
        viewModel.unreadOnly.toggle()
      }
      .accessibilityLabel("Unread, \(viewModel.unreadCount) items")
  }
}
