import Foundation
import TownCrierDomain

final class SpyRemoteNotificationRegistering: RemoteNotificationRegistering, @unchecked Sendable {
  private(set) var registerForRemoteNotificationsCallCount = 0

  func registerForRemoteNotifications() {
    registerForRemoteNotificationsCallCount += 1
  }
}
