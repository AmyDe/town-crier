import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("NotificationsView")
@MainActor
struct NotificationsViewTests {

  // MARK: - Helpers

  private func makeViewModel(
    notifications: [NotificationItem] = [],
    total: Int? = nil
  ) -> (NotificationsViewModel, SpyNotificationRepository) {
    let spy = SpyNotificationRepository()
    spy.fetchResult = .success(
      NotificationPage(
        notifications: notifications,
        total: total ?? notifications.count,
        page: 1
      )
    )
    let vm = NotificationsViewModel(repository: spy)
    return (vm, spy)
  }

  // MARK: - View Construction

  @Test("NotificationsView can be constructed with empty state")
  func construction_emptyState_succeeds() {
    let (vm, _) = makeViewModel()

    let view = NotificationsView(viewModel: vm)

    _ = view
  }

  @Test("NotificationsView can be constructed with notifications")
  func construction_withNotifications_succeeds() {
    let (vm, _) = makeViewModel(notifications: [.rearExtension, .changeOfUse])

    let view = NotificationsView(viewModel: vm)

    _ = view
  }

  // MARK: - Row Badge

  @Test("NotificationRow exposes a decision badge for DecisionUpdate items")
  func notificationRow_decisionUpdate_hasBadge() {
    let row = NotificationRow(item: .permittedDecision)

    #expect(row.shouldShowDecisionBadge)
  }

  @Test("NotificationRow hides badge for NewApplication items")
  func notificationRow_newApplication_hidesBadge() {
    let row = NotificationRow(item: .rearExtension)

    #expect(!row.shouldShowDecisionBadge)
  }

  @Test("NotificationRow hides badge when DecisionUpdate has unknown decision")
  func notificationRow_unknownDecision_hidesBadge() {
    let row = NotificationRow(item: .unknownDecision)

    #expect(!row.shouldShowDecisionBadge)
  }
}
