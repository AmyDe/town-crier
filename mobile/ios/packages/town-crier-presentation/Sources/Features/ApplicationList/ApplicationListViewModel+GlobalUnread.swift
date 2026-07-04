import Foundation

/// Best-effort refresh of the global unread count that gates the
/// Mark-All-Read toolbar button (tc-c5m1, GH#793). Split out of
/// `ApplicationListViewModel` to keep that file under SwiftLint's
/// `file_length` ceiling.
extension ApplicationListViewModel {

  /// Refreshes `globalUnreadCount` from the notification-state repository.
  /// Swallows all errors and never touches `error`/`isLoading` — this is a
  /// background enrichment of the Mark-All-Read button's visibility, not the
  /// primary load. No-ops when no repository was injected (count stays 0).
  func refreshGlobalUnread() async {
    guard let notificationStateRepository else { return }
    do {
      let state = try await notificationStateRepository.fetchState()
      globalUnreadCount = state.totalUnreadCount
    } catch {
      // Swallow — best-effort background enrichment; a later load retries.
    }
  }
}
