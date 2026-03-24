import TownCrierDomain

/// Bridges notification permission (from a platform-specific provider like UNUserNotificationCenter)
/// with device token registration (from the API).
/// Conforms to `NotificationService` so it can be injected wherever the domain protocol is required.
public final class CompositeNotificationService: NotificationService, @unchecked Sendable {
    private let permissionProvider: any NotificationPermissionProvider
    private let apiService: APINotificationService

    public init(
        permissionProvider: any NotificationPermissionProvider,
        apiService: APINotificationService
    ) {
        self.permissionProvider = permissionProvider
        self.apiService = apiService
    }

    public func requestPermission() async throws -> Bool {
        try await permissionProvider.requestPermission()
    }

    public func registerDeviceToken(_ token: String) async throws {
        try await apiService.registerDeviceToken(token)
    }

    public func removeDeviceToken() async throws {
        try await apiService.removeDeviceToken()
    }
}
