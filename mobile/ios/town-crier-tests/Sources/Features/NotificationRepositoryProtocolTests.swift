import Foundation
import Testing
import TownCrierDomain

@Suite("NotificationRepository protocol")
struct NotificationRepositoryProtocolTests {

  @Test("protocol defines fetch with page and pageSize")
  func protocolDefinesFetch() async throws {
    let spy = SpyNotificationRepository()
    let result = try await spy.fetch(page: 1, pageSize: 20)

    #expect(result.notifications.isEmpty)
    #expect(result.total == 0)
    #expect(result.page == 1)
  }

  @Test("spy records fetch calls")
  func spyRecordsFetchCalls() async throws {
    let spy = SpyNotificationRepository()
    _ = try await spy.fetch(page: 2, pageSize: 10)

    #expect(spy.fetchCalls.count == 1)
    #expect(spy.fetchCalls[0].page == 2)
    #expect(spy.fetchCalls[0].pageSize == 10)
  }

  @Test("spy can return configured result")
  func spyReturnsConfiguredResult() async throws {
    let spy = SpyNotificationRepository()
    let date = Date(timeIntervalSince1970: 1_712_000_000)
    let item = NotificationItem(
      applicationName: "Test",
      applicationAddress: "Address",
      applicationDescription: "Description",
      applicationType: "Type",
      authorityId: 1,
      createdAt: date,
      eventType: "NewApplication",
      decision: nil,
      sources: "Zone"
    )
    spy.fetchResult = .success(NotificationPage(notifications: [item], total: 1, page: 1))

    let result = try await spy.fetch(page: 1, pageSize: 20)

    #expect(result.notifications.count == 1)
    #expect(result.total == 1)
  }

  @Test("spy can throw configured error")
  func spyThrowsConfiguredError() async {
    let spy = SpyNotificationRepository()
    spy.fetchResult = .failure(DomainError.networkUnavailable)

    do {
      _ = try await spy.fetch(page: 1, pageSize: 20)
      Issue.record("Expected error to be thrown")
    } catch {
      #expect(error is DomainError)
    }
  }
}
