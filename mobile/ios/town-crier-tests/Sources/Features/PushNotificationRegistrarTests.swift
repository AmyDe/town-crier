import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("PushNotificationRegistrar")
struct PushNotificationRegistrarTests {
  private func makeSUT(
    session: AuthSession? = .valid
  ) -> (PushNotificationRegistrar, SpyNotificationService, SpyAuthenticationService) {
    let notificationService = SpyNotificationService()
    let authService = SpyAuthenticationService()
    authService.currentSessionResult = session
    let sut = PushNotificationRegistrar(
      notificationService: notificationService,
      authService: authService
    )
    return (sut, notificationService, authService)
  }

  // MARK: - didReceiveDeviceToken — authenticated

  @Test("didReceiveDeviceToken when authenticated registers hex token immediately")
  func didReceiveDeviceToken_whenAuthenticated_registersHexToken() async {
    let (sut, notificationService, _) = makeSUT(session: .valid)
    let tokenData = Data([0xDE, 0xAD, 0xBE, 0xEF])

    await sut.didReceiveDeviceToken(tokenData)

    #expect(notificationService.registerDeviceTokenCalls == ["deadbeef"])
  }
}
