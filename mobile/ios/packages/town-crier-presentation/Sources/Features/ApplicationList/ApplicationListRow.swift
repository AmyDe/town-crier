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
        application.status.displayLabel
    }

    private var statusIcon: String {
        application.status.displayIcon
    }

    private var statusColor: Color {
        application.status.displayColor
    }
}
