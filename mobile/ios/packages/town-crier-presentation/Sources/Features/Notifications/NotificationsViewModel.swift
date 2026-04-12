import Foundation
import TownCrierDomain

/// ViewModel driving the notifications list with pagination.
///
/// Notifications are available to all tiers -- no entitlement gating required.
/// Follows the same pagination pattern as ``SearchViewModel``: initial load
/// fetches page 1, and `loadMore()` appends subsequent pages.
@MainActor
public final class NotificationsViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published private(set) var notifications: [NotificationItem] = []
  @Published private(set) var isLoading = false
  @Published private(set) var total = 0
  @Published var error: DomainError?

  /// Whether a load has been performed (distinguishes "no results" from "not yet loaded").
  @Published private(set) var hasLoaded = false

  private let repository: NotificationRepository
  private let pageSize: Int
  private var pagination = PaginationState()

  /// Whether more pages of results are available.
  public var hasMore: Bool {
    pagination.hasMore
  }

  /// Whether the load returned zero results after a completed load.
  public var isEmpty: Bool {
    hasLoaded && notifications.isEmpty && error == nil && !isLoading
  }

  public init(repository: NotificationRepository, pageSize: Int = 20) {
    self.repository = repository
    self.pageSize = pageSize
  }

  /// Loads or refreshes the first page of notifications, resetting pagination.
  public func loadNotifications() async {
    isLoading = true
    error = nil
    notifications = []
    pagination.reset()
    hasLoaded = true

    do {
      let page = try await repository.fetch(page: 1, pageSize: pageSize)
      notifications = page.notifications
      total = page.total
      pagination.startInitialLoad(hasMore: page.hasMore)
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  /// Loads the next page of results, appending to the current list.
  public func loadMore() async {
    guard hasMore else { return }

    isLoading = true

    do {
      let page = try await repository.fetch(page: pagination.nextPage, pageSize: pageSize)
      notifications.append(contentsOf: page.notifications)
      total = page.total
      pagination.advance(hasMore: page.hasMore)
    } catch {
      handleError(error)
    }
    isLoading = false
  }
}
