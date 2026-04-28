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

  // MARK: - didReceiveDeviceToken — anonymous

  @Test("didReceiveDeviceToken when not authenticated does not call backend")
  func didReceiveDeviceToken_whenAnonymous_doesNotCallBackend() async {
    let (sut, notificationService, _) = makeSUT(session: nil)
    let tokenData = Data([0x01, 0x02, 0x03])

    await sut.didReceiveDeviceToken(tokenData)

    #expect(notificationService.registerDeviceTokenCalls.isEmpty)
  }

  // MARK: - flushPendingRegistration

  @Test("flushPendingRegistration after queued token registers it once user is authenticated")
  func flushPendingRegistration_afterQueuedToken_registersIt() async {
    let (sut, notificationService, authService) = makeSUT(session: nil)
    let tokenData = Data([0xCA, 0xFE, 0xBA, 0xBE])
    await sut.didReceiveDeviceToken(tokenData)
    #expect(notificationService.registerDeviceTokenCalls.isEmpty)

    // User signs in: simulate by switching the spy session.
    authService.currentSessionResult = .valid

    await sut.flushPendingRegistration()

    #expect(notificationService.registerDeviceTokenCalls == ["cafebabe"])
  }
}
