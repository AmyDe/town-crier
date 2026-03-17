import SwiftUI

/// Notification permission step — asks user to enable push notifications.
struct NotificationPermissionStepView: View {
    @ObservedObject var viewModel: OnboardingViewModel

    var body: some View {
        VStack(spacing: TCSpacing.large) {
            Image(systemName: "bell.badge")
                .font(.system(.largeTitle))
                .foregroundStyle(Color.tcAmber)

            Text("Stay Updated")
                .font(TCTypography.displaySmall)
                .foregroundStyle(Color.tcTextPrimary)

            Text("Get notified when new planning applications appear near you.")
                .font(TCTypography.body)
                .foregroundStyle(Color.tcTextSecondary)
                .multilineTextAlignment(.center)

            Button {
                Task { await viewModel.requestNotificationPermission() }
            } label: {
                Text("Enable Notifications")
                    .font(TCTypography.bodyEmphasis)
                    .frame(maxWidth: .infinity)
                    .frame(height: 44)
            }
            .buttonStyle(.borderedProminent)
            .tint(Color.tcAmber)
            .foregroundStyle(Color.tcTextOnAccent)
            .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))

            Button {
                Task { await viewModel.skipNotifications() }
            } label: {
                Text("Skip for Now")
                    .font(TCTypography.body)
                    .foregroundStyle(Color.tcTextSecondary)
            }

            Button("Back") {
                viewModel.goBack()
            }
            .font(TCTypography.body)
            .foregroundStyle(Color.tcTextSecondary)
        }
        .padding(TCSpacing.extraLarge)
    }
}
