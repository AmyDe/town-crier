import Foundation
import TownCrierDomain

/// Per-application "tap to read" mark-read flow (ADR 0035). Split out of
/// `ApplicationListViewModel` to keep that file under SwiftLint's
/// `file_length` ceiling.
extension ApplicationListViewModel {

  /// Fires a per-application mark-read when the opened row shows an unread
  /// badge, optimistically clearing that badge locally so ``unreadCount`` and
  /// the Unread chip update without a refetch. Already-read/absent rows and
  /// non-numeric authorities issue no request. Errors are swallowed — a later
  /// fetch reconciles (ADR 0035). The composite mirrors the deep-link parser:
  /// `applicationUid` is `id.name`; `authorityId` is `Int(id.authority)`.
  func markReadOnOpen(_ id: PlanningApplicationId) {
    guard let notificationStateRepository,
      let index = applications.firstIndex(where: { $0.id == id }),
      applications[index].latestUnreadEvent != nil,
      let authorityId = Int(id.authority)
    else {
      return
    }
    let applicationUid = id.name
    applications[index] = applications[index].withLatestUnreadEvent(nil)
    // Also push the OS app-icon badge, mirroring `markAllRead()`'s clear —
    // otherwise the badge only catches up on the next foreground sync, i.e.
    // after a relaunch (tc-4x8e0).
    globalUnreadCount = max(0, globalUnreadCount - 1)
    badgeSetter?.setBadge(globalUnreadCount)
    pendingMarkRead = Task { [weak self] in
      do {
        try await notificationStateRepository.markApplicationRead(
          applicationUid: applicationUid,
          authorityId: authorityId
        )
      } catch {
        // Swallow — optimistic UI; a later fetch reconciles (ADR 0035).
        _ = self
      }
    }
  }

  /// Test-only: await the most recent tap-to-read mark-read.
  public func waitForPendingMarkRead() async {
    await pendingMarkRead?.value
  }
}
