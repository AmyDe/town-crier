/// Abstracts the system's notification permission request (e.g. UNUserNotificationCenter)
/// so that code depending on it can be tested without a live notification centre.
public protocol NotificationPermissionProvider: Sendable {
    func requestPermission() async throws -> Bool
}
