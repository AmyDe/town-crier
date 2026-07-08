import Testing

@testable import TownCrierPresentation

@MainActor
@Suite("WelcomeView")
struct WelcomeViewTests {
  @Test func body_renders() {
    let sut = WelcomeView(viewModel: WelcomeViewModel())
    _ = sut.body
  }

  @Test func body_renders_withCallbacksWired() {
    let viewModel = WelcomeViewModel(onGetStarted: {}, onSignIn: {})
    let sut = WelcomeView(viewModel: viewModel)
    _ = sut.body
  }
}
