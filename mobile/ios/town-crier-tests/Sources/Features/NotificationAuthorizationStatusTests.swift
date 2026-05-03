import Testing
import TownCrierDomain

@Suite("NotificationAuthorizationStatus")
struct NotificationAuthorizationStatusTests {

  @Test("three cases are defined and distinct")
  func threeDistinctCases() {
    let notDetermined: NotificationAuthorizationStatus = .notDetermined
    let denied: NotificationAuthorizationStatus = .denied
    let authorized: NotificationAuthorizationStatus = .authorized

    #expect(notDetermined != denied)
    #expect(denied != authorized)
    #expect(notDetermined != authorized)
    #expect(notDetermined == .notDetermined)
  }
}
