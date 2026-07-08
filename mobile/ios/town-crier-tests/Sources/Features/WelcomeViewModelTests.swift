import Foundation
import Testing

@testable import TownCrierPresentation

@Suite("WelcomeViewModel")
@MainActor
struct WelcomeViewModelTests {
  @Test func getStarted_invokesOnGetStarted() {
    var invoked = false
    let sut = WelcomeViewModel(onGetStarted: { invoked = true }, onSignIn: {})

    sut.getStarted()

    #expect(invoked)
  }

  @Test func signIn_invokesOnSignIn() {
    var invoked = false
    let sut = WelcomeViewModel(onGetStarted: {}, onSignIn: { invoked = true })

    sut.signIn()

    #expect(invoked)
  }

  @Test func missingCallbacks_areNoOps() {
    let sut = WelcomeViewModel()

    sut.getStarted()
    sut.signIn()
    // No crash, nothing to assert beyond reaching this point.
  }
}
