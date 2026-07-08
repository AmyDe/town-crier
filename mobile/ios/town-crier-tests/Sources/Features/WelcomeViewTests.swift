import Foundation
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

  // MARK: - Appearance menu (GH#878)

  @Test func body_renders_withAppearanceStoreWired() {
    // swiftlint:disable:next force_unwrapping
    let store = AppearanceStore(defaults: UserDefaults(suiteName: UUID().uuidString)!)
    let viewModel = WelcomeViewModel(appearanceStore: store, onGetStarted: {}, onSignIn: {})
    let sut = WelcomeView(viewModel: viewModel)
    _ = sut.body
  }
}
