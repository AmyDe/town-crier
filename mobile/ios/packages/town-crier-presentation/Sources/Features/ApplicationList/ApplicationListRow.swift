import SwiftUI
import TownCrierDomain

/// A single row in the application list showing summary information.
struct ApplicationListRow: View {
    let application: PlanningApplication

    var body: some View {
        VStack(alignment: .leading, spacing: TCSpacing.small) {
            HStack {
                StatusBadgeView(status: application.status)
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
