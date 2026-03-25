import TownCrierDomain
import UserNotifications

/// Adapts `UNUserNotificationCenter` to the `NotificationPermissionProvider` protocol,
/// bridging the system notification permission request into the domain layer.
struct UNNotificationPermissionProvider: NotificationPermissionProvider {
    func requestPermission() async throws -> Bool {
        try await UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound, .badge])
    }
}
