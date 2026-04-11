import SwiftUI
import TownCrierDomain

/// List of planning applications the user has saved/bookmarked.
///
/// Available to all tiers. Shows a chronological list of saved applications
/// with swipe-to-unsave, empty state, and error handling following
/// the same pattern as ``NotificationsView``.
public struct SavedApplicationsView: View {
  @StateObject private var viewModel: SavedApplicationsViewModel

  public init(viewModel: SavedApplicationsViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    ZStack {
      Color.tcBackground.ignoresSafeArea()

      content
    }
    .navigationTitle("Saved")
    #if os(iOS)
      .navigationBarTitleDisplayMode(.large)
    #endif
    .task {
      if viewModel.savedApplications.isEmpty && !viewModel.hasLoaded {
        await viewModel.loadSavedApplications()
      }
    }
  }

  // MARK: - Content

  @ViewBuilder
  private var content: some View {
    if viewModel.isLoading && viewModel.savedApplications.isEmpty {
      ListSkeletonView()
    } else if let error = viewModel.error, viewModel.savedApplications.isEmpty {
      ErrorStateView(error: error) {
        await viewModel.loadSavedApplications()
      }
    } else if viewModel.isEmpty {
      EmptyStateView(
        icon: "bookmark.slash",
        title: "No Saved Applications",
        description:
          "Applications you save will appear here. Tap the bookmark icon on any application to save it."
      )
    } else {
      savedList
    }
  }

  // MARK: - List

  private var savedList: some View {
    List {
      ForEach(viewModel.savedApplications, id: \.applicationUid) { saved in
        SavedApplicationRow(saved: saved) {
          viewModel.selectApplication(uid: saved.applicationUid)
        }
        .listRowBackground(Color.tcSurface)
        .swipeActions(edge: .trailing, allowsFullSwipe: true) {
          Button(role: .destructive) {
            Task { await viewModel.unsave(applicationUid: saved.applicationUid) }
          } label: {
            Label("Unsave", systemImage: "bookmark.slash")
          }
        }
      }
    }
    .listStyle(.plain)
    .refreshable {
      await viewModel.loadSavedApplications()
    }
  }
}

// MARK: - Saved Application Row

/// A single row in the saved applications list showing the application summary.
private struct SavedApplicationRow: View {
  let saved: SavedApplication
  let onTap: () -> Void

  var body: some View {
    Button(action: onTap) {
      VStack(alignment: .leading, spacing: TCSpacing.small) {
        HStack {
          Image(systemName: "bookmark.fill")
            .font(TCTypography.caption)
            .foregroundStyle(Color.tcAmber)

          if let app = saved.application {
            StatusBadgeView(status: app.status)
          }

          Spacer()

          Text(saved.savedAt.formattedForDisplay)
            .font(TCTypography.caption)
            .foregroundStyle(Color.tcTextSecondary)
        }

        if let app = saved.application {
          Text(app.description)
            .font(TCTypography.headline)
            .foregroundStyle(Color.tcTextPrimary)
            .lineLimit(2)

          Text(app.address)
            .font(TCTypography.body)
            .foregroundStyle(Color.tcTextSecondary)
            .lineLimit(1)

          Text(app.authority.name)
            .font(TCTypography.caption)
            .foregroundStyle(Color.tcTextTertiary)
        } else {
          Text(saved.applicationUid)
            .font(TCTypography.headline)
            .foregroundStyle(Color.tcTextPrimary)
            .lineLimit(1)
        }
      }
      .padding(.vertical, TCSpacing.small)
    }
    .buttonStyle(.plain)
  }
}
