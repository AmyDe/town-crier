import SwiftUI

/// Postcode entry step. The user enters any UK postcode they want to watch, not
/// only their home address (tc-rae0).
struct PostcodeEntryStepView: View {
  /// User-facing copy, kept in one place so it can be unit-tested.
  enum Copy {
    static let title = "Pick a postcode to watch"
    static let helper =
      "Any UK postcode works. Your home, your work, a relative's street, or a "
      + "site you're keeping an eye on. We'll find planning applications nearby."
    static let placeholder = "e.g. CB1 2AD"
  }

  @ObservedObject var viewModel: OnboardingViewModel

  var body: some View {
    VStack(spacing: TCSpacing.large) {
      Text(Copy.title)
        .font(TCTypography.displaySmall)
        .foregroundStyle(Color.tcTextPrimary)

      Text(Copy.helper)
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        .multilineTextAlignment(.center)

      TextField(Copy.placeholder, text: $viewModel.postcodeInput)
        .textFieldStyle(.roundedBorder)
        .autocorrectionDisabled()
        #if os(iOS)
          .textInputAutocapitalization(.characters)
        #endif

      if let error = viewModel.error {
        Text(error.userMessage)
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcStatusRejected)
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
