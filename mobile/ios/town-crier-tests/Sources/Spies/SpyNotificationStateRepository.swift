import Foundation
import TownCrierDomain

/// One recorded `markApplicationRead` invocation, capturing the composite the
/// spy was called with so tests can assert the exact `(uid, authorityId)` pair.
struct MarkApplicationReadCall: Equatable {
  let applicationUid: String
  let authorityId: Int
}

final class SpyNotificationStateRepository: NotificationStateRepository, @unchecked Sendable {
  private(set) var fetchStateCallCount = 0
  var fetchStateResult: Result<NotificationState, Error> = .success(
    NotificationState(
      lastReadAt: Date(timeIntervalSince1970: 0),
      version: 1,
      totalUnreadCount: 0
    )
  )

  func fetchState() async throws -> NotificationState {
    fetchStateCallCount += 1
    return try fetchStateResult.get()
  }

  private(set) var markAllReadCallCount = 0
  var markAllReadResult: Result<Void, Error> = .success(())

  func markAllRead() async throws {
    markAllReadCallCount += 1
    try markAllReadResult.get()
  }

  private(set) var markApplicationReadCalls: [MarkApplicationReadCall] = []
  var markApplicationReadResult: Result<Void, Error> = .success(())

  func markApplicationRead(applicationUid: String, authorityId: Int) async throws {
    markApplicationReadCalls.append(
      MarkApplicationReadCall(applicationUid: applicationUid, authorityId: authorityId)
    )
    try markApplicationReadResult.get()
  }

  private(set) var advanceCalls: [Date] = []
  var advanceResult: Result<Void, Error> = .success(())

  func advance(asOf: Date) async throws {
    advanceCalls.append(asOf)
    try advanceResult.get()
  }
}
