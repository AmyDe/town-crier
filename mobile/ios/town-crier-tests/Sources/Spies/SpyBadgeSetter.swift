import Foundation
import TownCrierDomain

/// Records `setBadge` invocations for ``BadgeSetting`` consumers. Used to
/// verify foreground badge sync (tc-1nsa.9, see spec
/// `docs/specs/notifications-unread-watermark.md#ios-badge-foreground-push`).
final class SpyBadgeSetter: BadgeSetting, @unchecked Sendable {
  private(set) var setBadgeCalls: [Int] = []

  func setBadge(_ count: Int) {
    setBadgeCalls.append(count)
  }
}
