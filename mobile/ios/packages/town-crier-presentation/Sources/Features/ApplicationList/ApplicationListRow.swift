import SwiftUI
import TownCrierDomain

/// A single row in the application list showing summary information.
struct ApplicationListRow: View {
    let application: PlanningApplication

    private static let dateFormatter: DateFormatter = {
        let formatter = DateFormatter()
        formatter.dateFormat = "d MMM yyyy"
        formatter.locale = Locale(identifier: "en_GB")
        formatter.timeZone = TimeZone(identifier: "UTC")
        return formatter
    }()

    var body: some View {
        VStack(alignment: .leading, spacing: TCSpacing.small) {
            HStack {
                statusBadge
                Spacer()
                Text(Self.dateFormatter.string(from: application.receivedDate))
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

    // MARK: - Status Badge

    private var statusBadge: some View {
        HStack(spacing: TCSpacing.extraSmall) {
            Image(systemName: statusIcon)
            Text(statusLabel)
        }
        .font(TCTypography.captionEmphasis)
        .foregroundStyle(statusColor)
        .padding(.horizontal, TCSpacing.small)
        .padding(.vertical, TCSpacing.extraSmall)
        .background(statusColor.opacity(0.15))
        .clipShape(Capsule())
    }

    private var statusLabel: String {
        switch application.status {
        case .underReview:
            "Pending"
        case .approved:
            "Approved"
        case .refused:
            "Refused"
        case .withdrawn:
            "Withdrawn"
        case .appealed:
            "Appealed"
        case .unknown:
            "Unknown"
        }
    }

    private var statusIcon: String {
        switch application.status {
        case .underReview:
            "clock"
        case .approved:
            "checkmark.circle"
        case .refused:
            "xmark.circle"
        case .withdrawn:
            "arrow.uturn.backward.circle"
        case .appealed:
            "exclamationmark.triangle"
        case .unknown:
            "questionmark.circle"
        }
    }

    private var statusColor: Color {
        switch application.status {
        case .underReview:
            .tcStatusPending
        case .approved:
            .tcStatusApproved
        case .refused:
            .tcStatusRefused
        case .withdrawn:
            .tcStatusWithdrawn
        case .appealed:
            .tcStatusAppealed
        case .unknown:
            .tcTextTertiary
        }
    }
}
