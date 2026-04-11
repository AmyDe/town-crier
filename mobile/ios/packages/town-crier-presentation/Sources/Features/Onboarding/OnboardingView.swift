import SwiftUI
import TownCrierDomain

/// Container view managing the onboarding step flow with animated transitions.
public struct OnboardingView: View {
  @StateObject private var viewModel: OnboardingViewModel

  public init(viewModel: OnboardingViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    ZStack {
      Color.tcBackground.ignoresSafeArea()

      VStack(spacing: 0) {
        stepIndicator
          .padding(.top, TCSpacing.medium)

        Spacer()

        Group {
          switch viewModel.currentStep {
          case .welcome:
            WelcomeStepView(onContinue: viewModel.advance)
          case .postcodeEntry:
            PostcodeEntryStepView(viewModel: viewModel)
          case .radiusPicker:
            RadiusPickerStepView(viewModel: viewModel)
          case .notificationPermission:
            NotificationPermissionStepView(viewModel: viewModel)
          }
        }
        .transition(
          .asymmetric(
            insertion: .move(edge: .trailing).combined(with: .opacity),
            removal: .move(edge: .leading).combined(with: .opacity)
          )
        )
        .animation(.spring(response: 0.4), value: viewModel.currentStep)

        Spacer()
      }
    }
  }

  private var stepIndicator: some View {
    HStack(spacing: TCSpacing.small) {
      ForEach(Array(OnboardingStep.allCases.enumerated()), id: \.offset) { index, step in
        Capsule()
          .fill(step == viewModel.currentStep ? Color.tcAmber : Color.tcBorder)
          .frame(height: 4)
          .animation(.spring(response: 0.3), value: viewModel.currentStep)
      }
    }
    .padding(.horizontal, TCSpacing.extraLarge)
  }
}
