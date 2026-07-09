import SwiftUI

/// Final onboarding step. Tier-aware and honest about what the user's plan
/// actually delivers (tc-w3cb.4):
///
/// - Paid tiers get instant alerts, so this asks to enable notifications.
/// - Free accounts get a weekly email digest only (no instant push), so the
///   copy says exactly that and shows a *light* upgrade nudge — no second full
///   paywall (the radius step already carries the in-context upsell).
struct NotificationPermissionStepView: View {
  @ObservedObject var viewModel: OnboardingViewModel

  var body: some View {
    VStack(spacing: TCSpacing.large) {
      if viewModel.deliversInstantAlerts {
        instantAlertsContent
      } else {
        weeklyDigestContent
      }

      Button("Back") {
        viewModel.goBack()
      }
      .font(TCTypography.body)
      .foregroundStyle(Color.tcTextSecondary)
    }
    .padding(TCSpacing.extraLarge)
  }

  /// Paid tiers: instant alerts are delivered, so offer to enable notifications.
  @ViewBuilder
  private var instantAlertsContent: some View {
    Image(systemName: "bell.badge")
      .font(TCTypography.displayLarge)
      .foregroundStyle(Color.tcAmber)

    Text("Stay updated")
      .font(TCTypography.displaySmall)
      .foregroundStyle(Color.tcTextPrimary)

    Text("Get an alert the moment a planning application in your watch zone changes.")
      .font(TCTypography.body)
      .foregroundStyle(Color.tcTextSecondary)
      .multilineTextAlignment(.center)

    PrimaryButton("Enable notifications") {
      Task { await viewModel.requestNotificationPermission() }
    }

    Button {
      Task { await viewModel.skipNotifications() }
    } label: {
      Text("Skip for now")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  /// Free tier: weekly email digest only. No instant push to promise, so no
  /// permission request — just an honest summary and a light upgrade nudge.
  @ViewBuilder
  private var weeklyDigestContent: some View {
    Image(systemName: "envelope")
      .font(TCTypography.displayLarge)
      .foregroundStyle(Color.tcAmber)

    Text("Stay updated")
      .font(TCTypography.displaySmall)
      .foregroundStyle(Color.tcTextPrimary)

    Text("Your free plan sends a weekly email digest of new planning applications near you.")
      .font(TCTypography.body)
      .foregroundStyle(Color.tcTextSecondary)
      .multilineTextAlignment(.center)

    Text("Want to know the moment something changes? Instant alerts come with Personal and Pro.")
      .font(TCTypography.caption)
      .foregroundStyle(Color.tcTextSecondary)
      .multilineTextAlignment(.center)

    PrimaryButton("Finish") {
      Task { await viewModel.skipNotifications() }
    }
  }
}
