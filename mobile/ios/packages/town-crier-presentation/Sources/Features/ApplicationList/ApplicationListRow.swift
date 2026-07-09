import SwiftUI
import TownCrierDomain

/// A single row in the application list showing summary information.
///
/// A leading-aligned 8pt accent dot signals unread state when the row has a
/// `latestUnreadEvent` from the watermark-aware applications endpoint. When
/// the row has been read the dot is replaced by a same-size transparent
/// placeholder so column alignment stays stable across read/unread rows.
/// This keeps the iOS row visually aligned with the web `ApplicationCard`
/// from tc-1nsa.11 (spec decision #6).
struct ApplicationListRow: View {
  let application: PlanningApplication

  /// Diameter of the unread indicator dot, per design language.
  private static let unreadDotSize: CGFloat = 8

  /// Whether the row will render a visible unread dot. Exposed for tests so
  /// they can assert read-state styling without scraping the SwiftUI render
  /// tree.
  var hasUnreadDot: Bool { application.latestUnreadEvent != nil }

  /// The status pill the row will render. Exposed for tests so they can
  /// assert vocabulary delegation without scraping the SwiftUI render tree.
  var statusPill: ApplicationStatusPill {
    ApplicationStatusPill(status: application.status)
  }

  var body: some View {
    HStack(alignment: .top, spacing: TCSpacing.small) {
      unreadIndicator

      VStack(alignment: .leading, spacing: TCSpacing.small) {
        // Mono header strip: the planning reference as a monospaced
        // metadata line, ahead of the status/date row (GH#857).
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
