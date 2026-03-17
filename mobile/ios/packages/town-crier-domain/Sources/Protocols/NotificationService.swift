/// Requests and checks push notification permission.
public protocol NotificationService: Sendable {
    func requestPermission() async throws -> Bool
}
