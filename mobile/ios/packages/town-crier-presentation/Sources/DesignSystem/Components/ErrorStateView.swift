import SwiftUI
import TownCrierDomain

/// Reusable error state component following the design language.
/// Shows error icon, title, message, and retry button for retryable errors.
public struct ErrorStateView: View {
    private let error: DomainError
    private let retryAction: (() async -> Void)?

    public init(error: DomainError, retryAction: (() async -> Void)? = nil) {
        self.error = error
        self.retryAction = retryAction
    }

    public var body: some View {
        VStack(spacing: TCSpacing.medium) {
            Image(systemName: iconName)
                .font(.system(.largeTitle))
                .foregroundStyle(iconColor)

            Text(error.userTitle)
                .font(TCTypography.headline)
                .foregroundStyle(Color.tcTextPrimary)

            Text(error.userMessage)
                .font(TCTypography.body)
                .foregroundStyle(Color.tcTextSecondary)
                .multilineTextAlignment(.center)

            if error.isRetryable, let retryAction {
                PrimaryButton(
                    action: { Task { await retryAction() } },
                    label: {
                        HStack(spacing: TCSpacing.extraSmall) {
                            Image(systemName: "arrow.clockwise")
                            Text("Try Again")
                        }
                    }
                )
            }
        }
        .padding(TCSpacing.extraLarge)
    }

    private var iconName: String {
        switch error {
        case .networkUnavailable:
            return "wifi.slash"
        case .sessionExpired:
            return "lock.circle"
        case .authenticationFailed:
            return "person.crop.circle.badge.exclamationmark"
        default:
            return "exclamationmark.triangle"
        }
    }

    private var iconColor: Color {
        switch error {
        case .networkUnavailable:
            return .tcStatusPending
        case .sessionExpired, .authenticationFailed:
            return .tcStatusRefused
        default:
            return .tcStatusRefused
        }
    }
}
