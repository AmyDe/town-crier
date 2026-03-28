import SwiftUI

/// Welcome screen — first step of onboarding.
struct WelcomeStepView: View {
    let onContinue: () -> Void

    var body: some View {
        VStack(spacing: TCSpacing.large) {
            Image(systemName: "bell.badge")
                .font(.system(.largeTitle))
                .foregroundStyle(Color.tcAmber)

            Text("Welcome to Town Crier")
                .font(TCTypography.displayLarge)
                .foregroundStyle(Color.tcTextPrimary)
                .multilineTextAlignment(.center)

            Text("Stay informed about planning applications near you.")
                .font(TCTypography.body)
                .foregroundStyle(Color.tcTextSecondary)
                .multilineTextAlignment(.center)

            PrimaryButton("Get Started", action: onContinue)
        }
        .padding(TCSpacing.extraLarge)
    }
}
