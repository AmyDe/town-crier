import SwiftUI
import TownCrierDomain

/// Paginated list of notifications about planning application events.
///
/// Available to all tiers. Shows a chronological list of notification items
/// with load-more pagination, empty state, and error handling following
/// the same pattern as ``SearchView``.
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
private struct NotificationRow: View {
  let item: NotificationItem

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
    }
    .padding(.vertical, TCSpacing.small)
  }
}
