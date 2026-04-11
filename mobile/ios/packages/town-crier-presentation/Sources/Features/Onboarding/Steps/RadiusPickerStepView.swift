import SwiftUI

/// Radius selection step — user picks how far around their postcode to monitor.
struct RadiusPickerStepView: View {
  @ObservedObject var viewModel: OnboardingViewModel

  private let radiusOptions: [Double] = [500, 1000, 2000, 5000]

  var body: some View {
    VStack(spacing: TCSpacing.large) {
      Text("How far?")
        .font(TCTypography.displaySmall)
        .foregroundStyle(Color.tcTextPrimary)

      Text("Choose a radius around your postcode to monitor for planning applications.")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        .multilineTextAlignment(.center)

      VStack(spacing: TCSpacing.small) {
        ForEach(radiusOptions, id: \.self) { radius in
          Button {
            viewModel.selectedRadiusMetres = radius
          } label: {
            HStack {
              Text(formatRadius(radius))
                .font(TCTypography.bodyEmphasis)
              Spacer()
              if viewModel.selectedRadiusMetres == radius {
                Image(systemName: "checkmark.circle.fill")
                  .foregroundStyle(Color.tcAmber)
              }
            }
            .padding(TCSpacing.medium)
            .background(Color.tcSurface)
            .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
            .overlay(
              RoundedRectangle(cornerRadius: TCCornerRadius.medium)
                .stroke(
                  viewModel.selectedRadiusMetres == radius
                    ? Color.tcAmber : Color.tcBorder,
                  lineWidth: 1
                )
            )
          }
          .buttonStyle(.plain)
          .foregroundStyle(Color.tcTextPrimary)
        }
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

}
