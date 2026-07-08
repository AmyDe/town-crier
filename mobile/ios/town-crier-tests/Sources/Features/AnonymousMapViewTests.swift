import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@MainActor
@Suite("AnonymousMapView")
struct AnonymousMapViewTests {
  @Test func body_renders() {
    let viewModel = AnonymousMapViewModel(
      repository: SpyAnonymousApplicationsRepository(), coordinate: .cambridge)
    let sut = AnonymousMapView(viewModel: viewModel)
    _ = sut.body
  }

  /// Compiles and renders with the stacked-cluster disambiguation sheet
  /// (GH#877) populated, exercising the `.sheet(item:onDismiss:)` wiring
  /// added alongside the existing summary sheet.
  @Test func body_renders_withStackedApplicationsPresented() {
    let viewModel = AnonymousMapViewModel(
      repository: SpyAnonymousApplicationsRepository(), coordinate: .cambridge)
    viewModel.selectStack([.pendingReview, .permitted])
    let sut = AnonymousMapView(viewModel: viewModel)

    _ = sut.body

    #expect(viewModel.stackedApplications?.applications == [.pendingReview, .permitted])
  }
}
