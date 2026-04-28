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

  @Test("flushPendingRegistration is idempotent — repeated calls do not re-POST")
  func flushPendingRegistration_idempotent() async {
    let (sut, notificationService, authService) = makeSUT(session: nil)
    await sut.didReceiveDeviceToken(Data([0xCA, 0xFE]))
    authService.currentSessionResult = .valid

    await sut.flushPendingRegistration()
    await sut.flushPendingRegistration()

    #expect(notificationService.registerDeviceTokenCalls.count == 1)
  }

  @Test("flushPendingRegistration with no queued token is a no-op")
  func flushPendingRegistration_noQueuedToken_noOp() async {
    let (sut, notificationService, _) = makeSUT(session: .valid)

    await sut.flushPendingRegistration()

    #expect(notificationService.registerDeviceTokenCalls.isEmpty)
  }

  @Test("flushPendingRegistration while still anonymous does not register and keeps token queued")
  func flushPendingRegistration_stillAnonymous_keepsQueued() async {
    let (sut, notificationService, authService) = makeSUT(session: nil)
    await sut.didReceiveDeviceToken(Data([0xAB, 0xCD]))

    await sut.flushPendingRegistration()
    #expect(notificationService.registerDeviceTokenCalls.isEmpty)

    // Now authenticate and re-flush — the queued token should still be there.
    authService.currentSessionResult = .valid
    await sut.flushPendingRegistration()

    #expect(notificationService.registerDeviceTokenCalls == ["abcd"])
  }

  // MARK: - Token rotation / re-registration on each launch

  @Test("didReceiveDeviceToken while authenticated re-POSTs on every call (token rotation)")
  func didReceiveDeviceToken_authenticated_rePostsEachTime() async {
    let (sut, notificationService, _) = makeSUT(session: .valid)

    await sut.didReceiveDeviceToken(Data([0x01, 0x02]))
    await sut.didReceiveDeviceToken(Data([0x03, 0x04]))

    #expect(notificationService.registerDeviceTokenCalls == ["0102", "0304"])
  }

  @Test("rotated token while anonymous overwrites the queued token")
  func didReceiveDeviceToken_rotatedWhileAnonymous_overwritesQueue() async {
    let (sut, notificationService, authService) = makeSUT(session: nil)

    await sut.didReceiveDeviceToken(Data([0x01, 0x02]))
    await sut.didReceiveDeviceToken(Data([0x03, 0x04]))

    authService.currentSessionResult = .valid
    await sut.flushPendingRegistration()

    #expect(notificationService.registerDeviceTokenCalls == ["0304"])
  }

  // MARK: - didFailToRegister

  @Test("didFailToRegister does not throw and does not call the backend")
  func didFailToRegister_noBackendCall() async {
    let (sut, notificationService, _) = makeSUT(session: .valid)
    let error = NSError(domain: "APNs", code: 3010, userInfo: nil)

    await sut.didFailToRegister(error: error)

    #expect(notificationService.registerDeviceTokenCalls.isEmpty)
  }
}
