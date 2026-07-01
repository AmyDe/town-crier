import Foundation
import Testing
import TownCrierDomain

@Suite("NotificationStateRepository protocol")
struct NotificationStateRepositoryProtocolTests {

  @Test("spy returns configured state from fetchState")
  func spy_returnsConfiguredState() async throws {
    let spy = SpyNotificationStateRepository()
    let date = Date(timeIntervalSince1970: 1_712_000_000)
    spy.fetchStateResult = .success(
      NotificationState(lastReadAt: date, version: 2, totalUnreadCount: 4))

    let result = try await spy.fetchState()

    #expect(result.lastReadAt == date)
    #expect(result.version == 2)
    #expect(result.totalUnreadCount == 4)
  }

  @Test("spy records fetchState calls")
  func spy_recordsFetchStateCalls() async throws {
    let spy = SpyNotificationStateRepository()
    _ = try await spy.fetchState()
    _ = try await spy.fetchState()

    #expect(spy.fetchStateCallCount == 2)
  }

  @Test("spy records markAllRead calls")
  func spy_recordsMarkAllReadCalls() async throws {
    let spy = SpyNotificationStateRepository()

    try await spy.markAllRead()

    #expect(spy.markAllReadCallCount == 1)
  }

  @Test("spy records markApplicationRead calls with the composite (uid, authorityId)")
  func spy_recordsMarkApplicationReadCalls() async throws {
    let spy = SpyNotificationStateRepository()

    try await spy.markApplicationRead(applicationUid: "22/1234/FUL", authorityId: 42)

    #expect(
      spy.markApplicationReadCalls == [
        MarkApplicationReadCall(applicationUid: "22/1234/FUL", authorityId: 42)
      ]
    )
  }

  @Test("spy throws configured error from fetchState")
  func spy_throwsConfiguredFetchStateError() async {
    let spy = SpyNotificationStateRepository()
    spy.fetchStateResult = .failure(DomainError.networkUnavailable)

    do {
      _ = try await spy.fetchState()
      Issue.record("Expected error to be thrown")
    } catch {
      #expect(error is DomainError)
    }
  }
}
