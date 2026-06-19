import SwiftUI

/// Radius selection step — user picks how far around their postcode to monitor.
///
/// Uses the same `Slider` paradigm as `WatchZoneEditorView`, bounded at the
/// tier's maximum radius so a free user cannot exceed their 2 km cap by
/// construction (tc-w3cb.2).
struct RadiusPickerStepView: View {
  @ObservedObject var viewModel: OnboardingViewModel

  var body: some View {
    VStack(spacing: TCSpacing.large) {
      Text("How far?")
        .font(TCTypography.displaySmall)
        .foregroundStyle(Color.tcTextPrimary)

      Text("Choose a radius around your postcode to monitor for planning applications.")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        .multilineTextAlignment(.center)

      radiusControl

      if viewModel.canUnlockLargerRadius {
        UnlockLargerZonesChip {
          viewModel.requestLargerRadiusUpgrade()
        }
      }

      if viewModel.showsLargeRadiusWarning {
        LargeRadiusWarningView()
      }

      PrimaryButton("Continue") {
        viewModel.confirmRadius()
      }

      Button("Back") {
        viewModel.goBack()
      }
      .font(TCTypography.body)
      .foregroundStyle(Color.tcTextSecondary)
    }
    .padding(TCSpacing.extraLarge)
  }

  private var radiusControl: some View {
    VStack(alignment: .leading, spacing: TCSpacing.small) {
      Text(formatRadius(viewModel.selectedRadiusMetres))
        .font(TCTypography.bodyEmphasis)
        .foregroundStyle(Color.tcTextPrimary)
        .frame(maxWidth: .infinity, alignment: .center)

      Slider(
        value: $viewModel.selectedRadiusMetres,
        in: 100...viewModel.maxRadiusMetres,
        step: 100
      )
      .tint(Color.tcAmber)
      .accessibilityLabel("Radius")
      .accessibilityValue(formatRadius(viewModel.selectedRadiusMetres))

      HStack {
        Text(formatRadius(100))
        Spacer()
        Text(formatRadius(viewModel.maxRadiusMetres))
      }
      .font(TCTypography.caption)
      .foregroundStyle(Color.tcTextSecondary)
    }
  }

}
