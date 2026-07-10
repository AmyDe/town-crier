import Testing

@testable import TownCrierPresentation

@MainActor
@Suite("AnonymousPostcodeEntryView")
struct AnonymousPostcodeEntryViewTests {
  private func makeViewModel() -> AnonymousPostcodeEntryViewModel {
    AnonymousPostcodeEntryViewModel(
      geocoder: SpyPostcodeGeocoder(),
      stateRepository: SpyAnonymousBrowseStateRepository()
    )
  }

  @Test func body_renders() {
    let sut = AnonymousPostcodeEntryView(viewModel: makeViewModel())
    _ = sut.body
  }

  @Test func body_renders_withError() {
    let viewModel = makeViewModel()
    viewModel.postcodeInput = "INVALID"
    let sut = AnonymousPostcodeEntryView(viewModel: viewModel)
    _ = sut.body
  }

  /// GH#912 Phase 4: the radius picker moved here from the (now-removed)
  /// anonymous map slider — a render smoke test with a non-default radius
  /// selected.
  @Test func body_renders_withSelectedRadius() {
    let viewModel = makeViewModel()
    viewModel.selectedRadiusMetres = 1500
    let sut = AnonymousPostcodeEntryView(viewModel: viewModel)
    _ = sut.body
  }
}
