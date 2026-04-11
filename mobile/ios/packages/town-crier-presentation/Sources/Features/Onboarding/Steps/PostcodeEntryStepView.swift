import SwiftUI

/// Postcode entry step — user enters their postcode to set up a watch zone.
struct PostcodeEntryStepView: View {
  @ObservedObject var viewModel: OnboardingViewModel

  var body: some View {
    VStack(spacing: TCSpacing.large) {
      Text("Where do you live?")
        .font(TCTypography.displaySmall)
        .foregroundStyle(Color.tcTextPrimary)

      Text("Enter your postcode so we can find planning applications near you.")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        .multilineTextAlignment(.center)

      TextField("e.g. CB1 2AD", text: $viewModel.postcodeInput)
        .textFieldStyle(.roundedBorder)
        .autocorrectionDisabled()
        #if os(iOS)
          .textInputAutocapitalization(.characters)
        #endif

      if let error = viewModel.error {
        Text(error.userMessage)
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcStatusRefused)
      }

      PrimaryButton {
        Task { await viewModel.submitPostcode() }
      } label: {
        if viewModel.isLoading {
          ProgressView()
        } else {
          Text("Continue")
        }
      }
      .disabled(viewModel.postcodeInput.isEmpty || viewModel.isLoading)

      Button("Back") {
        viewModel.goBack()
      }
      .font(TCTypography.body)
      .foregroundStyle(Color.tcTextSecondary)
    }
    .padding(TCSpacing.extraLarge)
  }
}
