import SwiftUI
import TownCrierDomain

/// A single row in the application list showing summary information.
///
/// The status badge is rendered by ``ApplicationStatusPill`` — saturated when
/// the row has a `latestUnreadEvent` from the watermark-aware applications
/// endpoint, muted once the user has read the underlying notification. This
/// keeps the iOS row visually aligned with the web `ApplicationCard` from
/// tc-1nsa.11 (spec decision #6).
struct ApplicationListRow: View {
  let application: PlanningApplication

  /// The status pill the row will render. Exposed for tests so they can
  /// assert read-state styling without scraping the SwiftUI render tree.
  var statusPill: ApplicationStatusPill {
    ApplicationStatusPill(
      status: application.status,
      isMuted: application.latestUnreadEvent == nil
    )
  }

  var body: some View {
    VStack(alignment: .leading, spacing: TCSpacing.small) {
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
    .padding(.vertical, TCSpacing.small)
  }
}
