import SwiftUI
import TownCrierDomain

/// Paginated list of notifications about planning application events.
///
/// Available to all tiers. Shows a chronological list of notification items
/// with load-more pagination, empty state, and error handling.
public struct NotificationsView: View {
  @StateObject private var viewModel: NotificationsViewModel

  public init(viewModel: NotificationsViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    ZStack {
      Color.tcBackground.ignoresSafeArea()

      content
    }
    .navigationTitle("Notifications")
    #if os(iOS)
      .navigationBarTitleDisplayMode(.large)
    #endif
    .task {
      if viewModel.notifications.isEmpty && !viewModel.hasLoaded {
        await viewModel.loadNotifications()
      }
    }
  }

  // MARK: - Content

  @ViewBuilder
  private var content: some View {
    if viewModel.isLoading && viewModel.notifications.isEmpty {
      ListSkeletonView()
    } else if let error = viewModel.error, viewModel.notifications.isEmpty {
      ErrorStateView(error: error) {
        await viewModel.loadNotifications()
      }
    } else if viewModel.isEmpty {
      EmptyStateView(
        icon: "bell.slash",
        title: "No Notifications",
        description:
          "You will see notifications here when there are updates to planning applications in your watch zones."
      )
    } else {
      notificationList
    }
  }

  // MARK: - List

  private var notificationList: some View {
    List {
      ForEach(Array(viewModel.notifications.enumerated()), id: \.offset) { _, item in
        NotificationRow(item: item)
          .listRowBackground(Color.tcSurface)
      }

      if viewModel.hasMore {
        Section {
          HStack {
            Spacer()
            if viewModel.isLoading {
              ProgressView()
            } else {
              Button("Load More") {
                Task { await viewModel.loadMore() }
              }
              .font(TCTypography.bodyEmphasis)
              .foregroundStyle(Color.tcAmber)
            }
            Spacer()
          }
          .frame(minHeight: 44)
        }
        .listRowBackground(Color.tcBackground)
      }
    }
    .listStyle(.plain)
    .refreshable {
      await viewModel.loadNotifications()
    }
  }
}

// MARK: - Notification Row

/// A single row in the notifications list showing application event details.
///
/// Internal (not `private`) so unit tests can probe the decision-badge gate
/// directly via ``shouldShowDecisionBadge`` without scraping a SwiftUI render
/// tree. The view body itself is built via ``NotificationDecisionBadge`` so
/// the visual chip and the gating helper share a single source of truth.
struct NotificationRow: View {
  let item: NotificationItem

  /// Whether this row will display the decision badge for `item`.
  ///
  /// Mirrors ``NotificationDecisionBadge/displayLabel(for:)`` so tests can
  /// assert visibility without rendering the SwiftUI tree.
  var shouldShowDecisionBadge: Bool {
    NotificationDecisionBadge.displayLabel(for: item) != nil
  }

  var body: some View {
    VStack(alignment: .leading, spacing: TCSpacing.small) {
      HStack {
        Image(systemName: "bell.fill")
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcAmber)

        Text(item.applicationType)
          .font(TCTypography.captionEmphasis)
          .foregroundStyle(Color.tcTextSecondary)

        Spacer()

        Text(item.createdAt.formattedForDisplay)
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
      }

      Text(item.applicationName)
        .font(TCTypography.headline)
        .foregroundStyle(Color.tcTextPrimary)
        .lineLimit(2)

      Text(item.applicationAddress)
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        .lineLimit(1)

      if !item.applicationDescription.isEmpty {
        Text(item.applicationDescription)
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextTertiary)
          .lineLimit(2)
      }

      NotificationDecisionBadge(item: item)
    }
    .padding(.vertical, TCSpacing.small)
  }
}
