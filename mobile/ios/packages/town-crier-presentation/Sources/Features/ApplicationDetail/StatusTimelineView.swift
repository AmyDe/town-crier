import SwiftUI
import TownCrierDomain

/// Visual timeline showing chronological status changes for a planning application.
struct StatusTimelineView: View {
  let items: [TimelineItem]

  var body: some View {
    VStack(alignment: .leading, spacing: 0) {
      ForEach(Array(items.enumerated()), id: \.offset) { index, item in
        HStack(alignment: .top, spacing: TCSpacing.small) {
          timelineIndicator(item: item, isLast: index == items.count - 1)
          itemContent(item)
        }
      }
    }
    .padding(TCSpacing.medium)
    .background(Color.tcSurface)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
  }

  // MARK: - Timeline Indicator

  private func timelineIndicator(item: TimelineItem, isLast: Bool) -> some View {
    VStack(spacing: 0) {
      ZStack {
        Circle()
          .fill(statusColor(for: item.status).opacity(item.isCurrent ? 0.15 : 0.08))
          .frame(width: 32, height: 32)

        Image(systemName: item.icon)
          .font(.system(.caption))
          .foregroundStyle(statusColor(for: item.status))
      }

      if !isLast {
        Rectangle()
          .fill(Color.tcBorder)
          .frame(width: 2)
          .frame(height: TCSpacing.large)
      }
    }
    .frame(width: 32)
  }

  // MARK: - Item Content

  private func itemContent(_ item: TimelineItem) -> some View {
    VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
      Text(item.label)
        .font(item.isCurrent ? TCTypography.bodyEmphasis : TCTypography.body)
        .foregroundStyle(item.isCurrent ? Color.tcTextPrimary : Color.tcTextSecondary)

      Text(item.dateFormatted)
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextTertiary)
    }
    .padding(.top, TCSpacing.extraSmall)
    .padding(.bottom, TCSpacing.small)
  }

  // MARK: - Color

  private func statusColor(for status: ApplicationStatus) -> Color {
    status.displayColor
  }
}
