import SwiftUI
import TownCrierDomain

/// A single row in the application list.
///
/// Read rows replace the unread dot with a same-size transparent placeholder so
/// column alignment stays stable across read and unread rows.
struct ApplicationListRow: View {
  let application: PlanningApplication

  private static let unreadDotSize: CGFloat = 8

  /// Exposed for tests, so they need not scrape the SwiftUI render tree.
  var hasUnreadDot: Bool { application.latestUnreadEvent != nil }

  /// Exposed for tests, so they need not scrape the SwiftUI render tree.
  var statusPill: ApplicationStatusPill {
    ApplicationStatusPill(status: application.status)
  }

  var body: some View {
    HStack(alignment: .top, spacing: TCSpacing.small) {
      unreadIndicator

      VStack(alignment: .leading, spacing: TCSpacing.small) {
        // Mono header strip: the planning reference as a monospaced
        // metadata line, ahead of the status/date row.
        Text(application.reference.value)
          .font(TCTypography.mono)
          .foregroundStyle(Color.tcTextSecondary)

        HStack {
          statusPill
          Spacer()
          Text(application.receivedDate.formattedForDisplay)
            .font(TCTypography.caption)
            .foregroundStyle(Color.tcTextSecondary)
        }

        Text(application.description)
          .font(TCTypography.headline)
          .foregroundStyle(Color.tcTextPrimary)
          .lineLimit(2)

        Text(application.address)
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
          .lineLimit(1)
      }
    }
    .padding(TCSpacing.medium)
    .noticeCardStyle(isUnread: hasUnreadDot)
  }

  @ViewBuilder
  private var unreadIndicator: some View {
    if hasUnreadDot {
      Circle()
        .fill(Color.tcAmber)
        .frame(width: Self.unreadDotSize, height: Self.unreadDotSize)
        .accessibilityLabel("Unread")
    } else {
      Color.clear
        .frame(width: Self.unreadDotSize, height: Self.unreadDotSize)
        .accessibilityHidden(true)
    }
  }
}
