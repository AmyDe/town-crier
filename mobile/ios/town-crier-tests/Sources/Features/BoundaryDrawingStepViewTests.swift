import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// `BoundaryDrawingStepView` (GH#1031, tc-6he3x.10). The step is a passive
/// renderer of `OnboardingViewModel` state, mirroring
/// `RadiusPickerStepViewTests`: these tests verify the view constructs and
/// its body evaluates across the states it must support, rather than
/// asserting on rendered content (no ViewInspector in this project).
@MainActor
@Suite("BoundaryDrawingStepView")
struct BoundaryDrawingStepViewTests {
  private func makeViewModel(tier: SubscriptionTier = .personal) async -> OnboardingViewModel {
    let vm = OnboardingViewModel(
      geocoder: SpyPostcodeGeocoder(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      subscriptionTier: tier
    )
    vm.advance()  // -> postcodeEntry
    vm.postcodeInput = "CB1 2AD"
    await vm.submitPostcode()  // -> radiusPicker
    vm.onUpgradeFlowCompleted = { vm.subscriptionTier = tier }
    await vm.reconcileTierAfterCustomShapeUpgrade()  // -> boundaryDrawing
    return vm
  }

  @Test func body_renders_withNoVertices() async {
    let sut = BoundaryDrawingStepView(viewModel: await makeViewModel())
    _ = sut.body
  }

  @Test func body_renders_withVertices() async throws {
    let vm = await makeViewModel()
    vm.addVertex(try Coordinate(latitude: 51.50, longitude: -0.10))
    vm.addVertex(try Coordinate(latitude: 51.51, longitude: -0.09))
    vm.addVertex(try Coordinate(latitude: 51.50, longitude: -0.09))

    let sut = BoundaryDrawingStepView(viewModel: vm)

    _ = sut.body
  }
}
