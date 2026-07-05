import Foundation
import TownCrierDomain

/// Foreground badge sync + push-tap per-application mark-read (tc-0sfx.3).
///
/// See ADR 0035 (`docs/adr/0035-per-application-notification-read-state.md`).
///
/// Both flows are deliberately fire-and-forget:
///
/// - `syncBadge` runs whenever the scene enters the foreground. A failure
///   leaves the existing OS-level badge in place; the next foreground entry
///   will retry.
/// - the push-tap mark-read runs immediately after a push deep-link is routed,
///   and on success reconciles the OS badge from the server right away rather
///   than waiting on the next scenePhase `syncBadge()` (tc-4x8e0). Marking
///   read is idempotent, so a transient failure is acceptable — a later
///   `fetchState` reconciles.
extension AppCoordinator {
  /// Fetches the current `NotificationState` and pushes `totalUnreadCount`
  /// into the application icon badge. Best-effort — errors are swallowed and
  /// logged; the badge is left untouched on failure.
  public func syncBadge() async {
    let task: Task<Void, Never> = Task { [weak self] in
      await self?.performBadgeSync()
    }
    pendingBadgeSync = task
    await task.value
  }

  /// Single entry point for push-notification taps. Parses the APNs `userInfo`
  /// payload; when it carries an application deep link, routes to that
  /// application's detail AND marks it read (tapping the push means the user
  /// has seen it — ADR 0035). A push with no application deep link (e.g. a
  /// digest push) simply no-ops: there is no single application to mark.
  public func handlePushTap(userInfo: [AnyHashable: Any]) {
    guard let deepLink = NotificationPayloadParser.parseDeepLink(from: userInfo) else {
      return
    }
    handleDeepLink(deepLink)
    if case .applicationDetail(let id) = deepLink {
      markPushedApplicationRead(id)
    }
  }

  /// Marks the deep-linked application read via the composite
  /// `(applicationUid, authorityId)`. `applicationUid` is the bare PlanIt ref
  /// (`id.name`); `authorityId` is `id.authority` (which is
  /// `String(authorityId)`) converted back to `Int`. A non-numeric authority
  /// no-ops. Stored as `pendingApplicationMarkRead` so tests can await it;
  /// production is fire-and-forget. On success, reconciles the OS badge from
  /// the server via `performBadgeSync()` rather than leaving it to race the
  /// next scenePhase-triggered `syncBadge()` (tc-4x8e0). A failed mark-read
  /// leaves the badge untouched — a retry or the next foreground sync
  /// reconciles.
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

  /// Test-only synchronisation: await the most recent push-tap mark-read so
  /// assertions can run after the repository call settles.
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
