/// Manages push notification permission and device token registration with the backend.
public protocol NotificationService: Sendable {
    func requestPermission() async throws -> Bool
    func registerDeviceToken(_ token: String) async throws
    func removeDeviceToken() async throws
}
