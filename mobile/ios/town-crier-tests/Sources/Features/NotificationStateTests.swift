import Foundation
import Testing
import TownCrierDomain

@Suite("NotificationState")
struct NotificationStateTests {

  @Test("init exposes lastReadAt, version and totalUnreadCount")
  func init_exposesProperties() {
    let date = Date(timeIntervalSince1970: 1_712_000_000)
    let state = NotificationState(lastReadAt: date, version: 3, totalUnreadCount: 7)

    #expect(state.lastReadAt == date)
    #expect(state.version == 3)
    #expect(state.totalUnreadCount == 7)
  }

  @Test("equal instances compare equal")
  func equality_sameValues_areEqual() {
    let date = Date(timeIntervalSince1970: 1_712_000_000)
    let lhs = NotificationState(lastReadAt: date, version: 1, totalUnreadCount: 5)
    let rhs = NotificationState(lastReadAt: date, version: 1, totalUnreadCount: 5)

    #expect(lhs == rhs)
  }

  @Test("instances with different totalUnreadCount are not equal")
  func equality_differentUnreadCount_notEqual() {
    let date = Date(timeIntervalSince1970: 1_712_000_000)
    let lhs = NotificationState(lastReadAt: date, version: 1, totalUnreadCount: 5)
    let rhs = NotificationState(lastReadAt: date, version: 1, totalUnreadCount: 6)

    #expect(lhs != rhs)
  }

  @Test("hasUnread is true when totalUnreadCount > 0")
  func hasUnread_isTrueWhenCountPositive() {
    let state = NotificationState(
      lastReadAt: Date(timeIntervalSince1970: 0), version: 1, totalUnreadCount: 1)

    #expect(state.hasUnread)
  }

  @Test("hasUnread is false when totalUnreadCount is zero")
  func hasUnread_isFalseWhenCountZero() {
    let state = NotificationState(
      lastReadAt: Date(timeIntervalSince1970: 0), version: 1, totalUnreadCount: 0)

    #expect(!state.hasUnread)
  }
}
