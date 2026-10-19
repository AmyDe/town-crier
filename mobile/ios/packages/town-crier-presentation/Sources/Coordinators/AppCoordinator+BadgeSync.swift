import Foundation
import TownCrierDomain

/// Foreground badge sync and push-tap mark-read. Both are deliberately
/// fire-and-forget: a failure leaves the OS badge as it was, and the next
/// foreground `syncBadge()` or `fetchState` reconciles it.
extension AppCoordinator {
  /// Sets the app icon badge to the server's `totalUnreadCount`. Best-effort:
  /// errors are logged and the badge is left untouched.
  public func syncBadge() async {
    let task: Task<Void, Never> = Task { [weak self] in
      await self?.performBadgeSync()
    }
    pendingBadgeSync = task
    await task.value
  }

  /// Routes a push tap's application deep link and marks that application
  /// read. A push without one (e.g. a digest push) no-ops.
  public func handlePushTap(userInfo: [AnyHashable: Any]) {
    guard let deepLink = NotificationPayloadParser.parseDeepLink(from: userInfo) else {
      return
    }
    handleDeepLink(deepLink)
    if case .applicationDetail(let id) = deepLink {
      markPushedApplicationRead(id)
    }
  }

  /// `id.authority` is `String(authorityId)`, so it is parsed back to `Int`;
  /// a non-numeric authority no-ops. On success the badge is reconciled
  /// immediately rather than racing the next scenePhase `syncBadge()`.
  private func markPushedApplicationRead(_ id: PlanningApplicationId) {
    guard let notificationStateRepository, let authorityId = Int(id.authority) else {
      return
    }
    let applicationUid = id.name
    pendingApplicationMarkRead = Task { [weak self] in
      do {
        try await notificationStateRepository.markApplicationRead(
          applicationUid: applicationUid,
          authorityId: authorityId
        )
        await self?.performBadgeSync()
      } catch {
        Self.logger.error(
          "Push-tap mark-read failed: \(error.localizedDescription)"
        )
      }
    }
  }

  /// Test-only synchronisation: await the most recent foreground badge sync.
  public func waitForPendingBadgeSync() async {
    await pendingBadgeSync?.value
  }

  /// Test-only synchronisation: await the most recent push-tap mark-read.
  public func waitForPendingApplicationMarkRead() async {
    await pendingApplicationMarkRead?.value
  }

  private func performBadgeSync() async {
    guard let notificationStateRepository else { return }
    do {
      let state = try await notificationStateRepository.fetchState()
      badgeSetter?.setBadge(state.totalUnreadCount)
    } catch {
      Self.logger.error(
        "Foreground badge sync failed: \(error.localizedDescription)"
      )
    }
  }
}
