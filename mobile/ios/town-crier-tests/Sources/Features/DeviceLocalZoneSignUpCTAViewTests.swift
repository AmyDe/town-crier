import SwiftUI
import Testing

@testable import TownCrierPresentation

@Suite("DeviceLocalZoneSignUpCTAView")
@MainActor
struct DeviceLocalZoneSignUpCTAViewTests {
  @Test func init_createsView() {
    let sut = DeviceLocalZoneSignUpCTAView(
      onCreateAccount: {}, onSignIn: {}, onDismiss: {})
    _ = sut.body
  }

  @Test func onCreateAccount_isCalled_whenCreateAccountTapped() {
    var called = false
    let sut = DeviceLocalZoneSignUpCTAView(
      onCreateAccount: { called = true }, onSignIn: {}, onDismiss: {})
    sut.simulateCreateAccountTap()
    #expect(called)
  }

  @Test func onSignIn_isCalled_whenSignInTapped() {
    var called = false
    let sut = DeviceLocalZoneSignUpCTAView(
      onCreateAccount: {}, onSignIn: { called = true }, onDismiss: {})
    sut.simulateSignInTap()
    #expect(called)
  }

  @Test func onDismiss_isCalled_whenNotNowTapped() {
    var called = false
    let sut = DeviceLocalZoneSignUpCTAView(
      onCreateAccount: {}, onSignIn: {}, onDismiss: { called = true })
    sut.simulateDismissTap()
    #expect(called)
  }
}
