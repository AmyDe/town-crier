import Foundation
import TownCrierDomain

final class SpyNotificationPermissionProvider: NotificationPermissionProvider, @unchecked Sendable {
  private(set) var requestPermissionCallCount = 0
  var requestPermissionResult: Result<Bool, Error> = .success(true)

  func requestPermission() async throws -> Bool {
    requestPermissionCallCount += 1
    return try requestPermissionResult.get()
  }

  private(set) var authorizationStatusCallCount = 0
  var nextAuthorizationStatus: NotificationAuthorizationStatus = .authorized

  func authorizationStatus() async -> NotificationAuthorizationStatus {
    authorizationStatusCallCount += 1
    return nextAuthorizationStatus
  }
}
