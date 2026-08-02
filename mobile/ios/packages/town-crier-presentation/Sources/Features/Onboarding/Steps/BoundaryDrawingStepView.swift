import SwiftUI
import TownCrierDomain

/// Custom-shape boundary drawing step (GH#1031, tc-6he3x.10) — reached only
/// after a mid-flow purchase from the radius step's custom-shape upsell
/// (``OnboardingViewModel/reconcileTierAfterCustomShapeUpgrade()``).
///
/// Mirrors ``RadiusPickerStepView``'s structure and styling, and mirrors
/// `WatchZoneEditorView`'s `boundaryDrawingSection` for the map/vertex-count/
/// Undo layout, but hosts ``BoundaryDrawingMapView`` bound to
/// ``OnboardingViewModel`` (rather than `WatchZoneEditorViewModel`) via the
/// shared ``BoundaryDrawingViewModel`` surface.
struct BoundaryDrawingStepView: View {
  @ObservedObject var viewModel: OnboardingViewModel

  var body: some View {
    VStack(spacing: TCSpacing.large) {
      Text("Draw your area")
        .font(TCTypography.displaySmall)
        .foregroundStyle(Color.tcTextPrimary)

      Text(
        "Tap the map to add points around the area you want to watch. "
          + "Tap the first point again to finish."
      )
      .font(TCTypography.body)
      .foregroundStyle(Color.tcTextSecondary)
      .multilineTextAlignment(.center)

      boundaryMap

      HStack {
        Text(vertexCountLabel)
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
        Spacer()
        Button("Undo") {
          viewModel.undoLastVertex()
        }
        .disabled(viewModel.boundaryVertices.isEmpty)
      }

      PrimaryButton("Continue") {
        viewModel.confirmBoundary()
      }
      .disabled(!viewModel.hasMinimumBoundaryVertices)

      Button("Back") {
        viewModel.goBack()
      }
      .font(TCTypography.body)
      .foregroundStyle(Color.tcTextSecondary)
    }
    .padding(TCSpacing.extraLarge)
  }

  @ViewBuilder
  private var boundaryMap: some View {
    if let coordinate = viewModel.geocodedCoordinate {
      #if canImport(UIKit)
        BoundaryDrawingMapView(viewModel: viewModel, initialCentre: coordinate)
          .frame(height: 260)
          .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
      #else
        // BoundaryDrawingMapView wraps UIKit's MKMapView and is unavailable
        // on this SPM compile-time-only macOS target (the app ships on iOS;
        // macOS here exists only so `swift build`/`swift test` can exercise
        // the shared code without Xcode).
        Text("Custom-shape drawing is available on iOS.")
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextSecondary)
      #endif
    }
  }

  private var vertexCountLabel: String {
    let count = viewModel.boundaryVertices.count
    if count < 3 {
      return "\(count) of at least 3 points"
    }
    return "\(count) points"
  }
}
