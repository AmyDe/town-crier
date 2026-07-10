import SwiftUI

/// Postcode entry step of the anonymous browse flow (GH#868 Phase 3) — same
/// styling/validation feel as the onboarding wizard's
/// `PostcodeEntryStepView`, but geocoding goes through
/// ``AnonymousPostcodeEntryViewModel`` (postcodes.io directly, no session)
/// rather than the wizard's authenticated `/v1/geocode`.
struct AnonymousPostcodeEntryView: View {
  enum Copy {
    static let title = "Where should we look?"
    static let helper =
      "Enter any UK postcode. We'll show planning applications nearby — no account needed."
    static let placeholder = "e.g. CB1 2AD"
  }

  @StateObject private var viewModel: AnonymousPostcodeEntryViewModel

  init(viewModel: AnonymousPostcodeEntryViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

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
        .font(TCTypography.mono)
        .textFieldStyle(.roundedBorder)
        .autocorrectionDisabled()
        #if os(iOS)
          .textInputAutocapitalization(.characters)
        #endif

      radiusControl

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

      Button("Back", action: viewModel.goBack)
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
    }
    .padding(TCSpacing.extraLarge)
    .frame(maxWidth: .infinity, maxHeight: .infinity)
    .background(Color.tcBackground)
  }

  // MARK: - Radius picker (GH#912 Phase 4)

  /// Mirrors the onboarding wizard's `RadiusPickerStepView.radiusControl` —
  /// same label/slider/min-max layout — so choosing a radius feels identical
  /// whether it happens here (pre-signup) or in the wizard (post-signup).
  /// Replaces the anonymous map's removed live slider as the sole way to set
  /// the initial monitoring radius.
  private var radiusControl: some View {
    VStack(alignment: .leading, spacing: TCSpacing.small) {
      Text(formatRadius(viewModel.selectedRadiusMetres))
        .font(TCTypography.bodyEmphasis)
        .foregroundStyle(Color.tcTextPrimary)
        .frame(maxWidth: .infinity, alignment: .center)

      Slider(
        value: $viewModel.selectedRadiusMetres,
        in: viewModel.minRadiusMetres...viewModel.maxRadiusMetres,
        step: 100
      )
      .tint(Color.tcAmber)
      .accessibilityLabel("Search radius")
      .accessibilityValue(formatRadius(viewModel.selectedRadiusMetres))

      HStack {
        Text(formatRadius(viewModel.minRadiusMetres))
        Spacer()
        Text(formatRadius(viewModel.maxRadiusMetres))
      }
      .font(TCTypography.caption)
      .foregroundStyle(Color.tcTextSecondary)
    }
  }
}
