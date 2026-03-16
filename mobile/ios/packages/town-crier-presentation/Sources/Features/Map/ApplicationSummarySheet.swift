import SwiftUI
import TownCrierDomain

/// A bottom sheet showing a summary of a selected planning application.
struct ApplicationSummarySheet: View {
    let application: PlanningApplication

    var body: some View {
        VStack(alignment: .leading, spacing: TCSpacing.medium) {
            HStack {
                statusBadge
                Spacer()
                Text(application.reference.value)
                    .font(.system(.caption))
                    .foregroundStyle(Color.tcTextSecondary)
            }

            Text(application.description)
                .font(.system(.headline, weight: .semibold))
                .foregroundStyle(Color.tcTextPrimary)

            Label(application.address, systemImage: "mappin.and.ellipse")
                .font(.system(.body))
                .foregroundStyle(Color.tcTextSecondary)

            Text("Received \(application.receivedDate.formatted(date: .abbreviated, time: .omitted))")
                .font(.system(.caption))
                .foregroundStyle(Color.tcTextTertiary)
        }
        .padding(TCSpacing.medium)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Color.tcSurfaceElevated)
        .presentationDetents([.medium, .fraction(0.3)])
        .presentationDragIndicator(.visible)
    }

    private var statusBadge: some View {
        HStack(spacing: TCSpacing.extraSmall) {
            Image(systemName: statusIcon)
            Text(statusLabel)
        }
        .font(.system(.caption, weight: .medium))
        .foregroundStyle(statusColor)
        .padding(.horizontal, TCSpacing.small)
        .padding(.vertical, TCSpacing.extraSmall)
        .background(statusColor.opacity(0.15))
        .clipShape(Capsule())
    }

    private var statusColor: Color {
        switch application.status {
        case .underReview: return .tcStatusPending
        case .approved: return .tcStatusApproved
        case .refused: return .tcStatusRefused
        case .withdrawn: return .tcStatusWithdrawn
        case .appealed: return .tcStatusAppealed
        case .unknown: return .tcTextTertiary
        }
    }

    private var statusLabel: String {
        switch application.status {
        case .underReview: return "Pending"
        case .approved: return "Approved"
        case .refused: return "Refused"
        case .withdrawn: return "Withdrawn"
        case .appealed: return "Appealed"
        case .unknown: return "Unknown"
        }
    }

    private var statusIcon: String {
        switch application.status {
        case .underReview: return "clock"
        case .approved: return "checkmark.circle"
        case .refused: return "xmark.circle"
        case .withdrawn: return "arrow.uturn.backward.circle"
        case .appealed: return "exclamationmark.triangle"
        case .unknown: return "questionmark.circle"
        }
    }
}
