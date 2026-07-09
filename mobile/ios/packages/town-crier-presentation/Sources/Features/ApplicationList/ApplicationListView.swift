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
/// - a Mark-All-Read toolbar action, visible whenever the app-icon badge has
///   something to clear (`hasClearableUnread`, the GLOBAL unread count) —
///   deliberately decoupled from the per-zone `hasUnread` chip so the badge
///   stays reachable even when the active zone has no unread rows of its own
///   (tc-c5m1, GH#793).
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
    .onChange(of: viewModel.sort) {
      // Server-driven sorts re-page from page 1 with a fresh cursor; switching
      // between the two client-side sorts only re-orders in memory (GH#682).
      Task { await viewModel.handleSortChanged() }
    }
    .onChange(of: viewModel.activeFilter) {
      // The status chips and Unread toggle now drive the server query, so a
      // filter change re-pages from page 1 with a fresh cursor (GH#682 slice 4).
      // Observing the derived `activeFilter` coalesces the chip + Unread updates
      // into a single reload even when one toggle clears the other.
      Task { await viewModel.handleFilterChanged() }
    }
  }

  // MARK: - Toolbar

  @ToolbarContentBuilder
  private var sortToolbarItem: some ToolbarContent {
    ToolbarItem(placement: sortToolbarPlacement) {
      Menu {
        Picker("Sort", selection: sortBinding) {
          ForEach(viewModel.availableSortOptions, id: \.self) { mode in
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
    if viewModel.hasClearableUnread {
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
          .cardRowInsets()
          .contentShape(Rectangle())
          .onTapGesture {
            viewModel.selectApplication(application.id)
          }
          .onAppear {
            // Infinite scroll: nearing the end pulls the next server page when
            // the active sort is server-driven (GH#682). A no-op otherwise.
            Task { await viewModel.onRowAppear(application) }
          }
      }
    }
  }

  // MARK: - Filter Chips

  private func filterChip(label: String, status: ApplicationStatus?) -> some View {
    // The extra `!unreadOnly` guard stays here, not in the shared
    // `CapsuleChipView`: when the Unread chip is active, every status chip
    // must read as unselected even if `selectedStatusFilter` still matches.
    let isSelected = !viewModel.unreadOnly && viewModel.selectedStatusFilter == status
    return CapsuleChipView(label: label, isSelected: isSelected) {
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
