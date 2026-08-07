import SwiftUI

/// Radius selection step — user picks how far around their postcode to monitor.
///
/// Uses the same `Slider` paradigm as `WatchZoneEditorView`, bounded at the
/// tier's maximum radius so a free user cannot exceed their 2 km cap by
/// construction (tc-w3cb.2).
///
/// Wrapped in a `ScrollView` (tc-7se1w.4): this step can stack the radius
/// control, the larger-radius upsell chip, the custom-shape card, and the
/// large-radius warning all at once, and `OnboardingView` gives the step
/// the full height below the step indicator rather than centring it — so on
/// a compact screen or with larger Dynamic Type, content that doesn't fit
/// scrolls instead of being silently clipped past the bottom of the
/// viewport. A tester who never scrolled would see the visible portion but
/// could miss anything pushed below the fold; before this fix there was no
/// way to reach it at all.
struct RadiusPickerStepView: View {
  @ObservedObject var viewModel: OnboardingViewModel

  var body: some View {
    ScrollView {
      VStack(spacing: TCSpacing.large) {
        Text("How far?")
          .font(TCTypography.displaySmall)
          .foregroundStyle(Color.tcTextPrimary)

        Text("Choose a radius around your postcode to monitor for planning applications.")
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextSecondary)
          .multilineTextAlignment(.center)

        radiusControl

        // Custom-shape mention (GH#1031, tc-6he3x.10; placement tc-7se1w.4):
        // shown directly below the radius control, ahead of the larger-radius
        // chip, so the newer polygon capability isn't the last thing a user
        // scrolls past. Free tier sees the upsell card; a tier that already
        // has the entitlement sees an equally clear card rather than a bare
        // text link (previously the only mention it got).
        if !viewModel.canDrawCustomShape {
          CustomShapeUpsellCard {
            viewModel.requestCustomShapeUpgrade()
          }
        } else {
          // Re-entry point for a user whose tier already allows custom shapes
          // (tc-6he3x.10) — without this, backing out of the drawing step once
          // would strand an already-paying user on a circle for the rest of
          // onboarding, since the upsell card above only shows pre-purchase.
          CustomShapeAvailableCard {
            viewModel.selectCustomShape()
          }
        }

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
      .frame(maxWidth: .infinity)
    }
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
