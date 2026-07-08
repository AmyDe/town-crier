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
}
