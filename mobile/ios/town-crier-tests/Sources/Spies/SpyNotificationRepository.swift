import TownCrierDomain

final class SpyNotificationRepository: NotificationRepository, @unchecked Sendable {
  struct FetchCall: Equatable {
    let page: Int
    let pageSize: Int
  }

  private(set) var fetchCalls: [FetchCall] = []
  var fetchResult: Result<NotificationPage, Error> = .success(
    NotificationPage(notifications: [], total: 0, page: 1)
  )

  func fetch(page: Int, pageSize: Int) async throws -> NotificationPage {
    fetchCalls.append(FetchCall(page: page, pageSize: pageSize))
    return try fetchResult.get()
  }
}
