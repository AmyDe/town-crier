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
}
