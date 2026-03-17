import SwiftUI
import TownCrierDomain

/// A banner indicating that displayed data may be outdated or the device is offline.
public struct StaleDataBanner: View {
    private let freshness: DataFreshness

    public init(freshness: DataFreshness) {
        self.freshness = freshness
    }

    public var body: some View {
        switch freshness {
        case .fresh:
            EmptyView()
        case .stale:
            banner(
                icon: "clock.arrow.circlepath",
                text: "Data may be outdated",
                background: Color.tcStatusPending.opacity(0.15)
            )
        case .offline:
            banner(
                icon: "wifi.slash",
                text: "You're offline — showing cached data",
                background: Color.tcStatusWithdrawn.opacity(0.15)
            )
        }
    }

    private func banner(icon: String, text: String, background: Color) -> some View {
        HStack(spacing: TCSpacing.small) {
            Image(systemName: icon)
                .font(.system(.caption))
                .foregroundStyle(Color.tcTextSecondary)
            Text(text)
                .font(.system(.caption))
                .foregroundStyle(Color.tcTextSecondary)
            Spacer()
        }
        .padding(.horizontal, TCSpacing.medium)
        .padding(.vertical, TCSpacing.small)
        .background(background)
        .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.small))
    }
}
