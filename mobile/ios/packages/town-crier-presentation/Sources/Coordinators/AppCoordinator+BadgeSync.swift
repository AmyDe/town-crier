import Foundation
import TownCrierDomain

/// Foreground badge sync + push-tap watermark advance (tc-1nsa.9).
///
/// Spec: docs/specs/notifications-unread-watermark.md#ios-badge-foreground-push.
///
/// The two flows are deliberately fire-and-forget:
///
/// - `syncBadge` runs whenever the scene enters the foreground. A failure
///   leaves the existing OS-level badge in place; the next foreground entry
///   will retry.
/// - `advanceWatermark` runs immediately after a push deep-link is routed.
///   The server enforces watermark monotonicity, so retries on stale instants
///   are no-ops and any transient failure is acceptable.
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

  /// Advances the read-state watermark to `asOf`. Stored as
  /// `pendingWatermarkAdvance` so tests can await deterministically without
  /// exposing the underlying repository call to the production caller, who
  /// is intentionally fire-and-forget.
  public func advanceWatermark(asOf: Date) {
    guard let notificationStateRepository else { return }
    pendingWatermarkAdvance = Task { [weak self] in
      do {
        try await notificationStateRepository.advance(asOf: asOf)
      } catch {
        Self.logger.error(
          "Watermark advance failed: \(error.localizedDescription)"
        )
        _ = self  // retain to keep ARC happy; nothing else to do
      }
    }
  }

  /// Test-only synchronisation: await the most recent foreground badge sync.
  public func waitForPendingBadgeSync() async {
    await pendingBadgeSync?.value
  }

  /// Test-only synchronisation: await the most recent push-tap watermark
  /// advance so assertions can run after the repository call settles.
  public func waitForPendingWatermarkAdvance() async {
    await pendingWatermarkAdvance?.value
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
