import TownCrierDomain

/// Bridges notification permission (from a platform-specific provider like UNUserNotificationCenter)
/// with device token registration (from the API).
/// Conforms to `NotificationService` so it can be injected wherever the domain protocol is required.
public final class CompositeNotificationService: NotificationService, @unchecked Sendable {
  private let permissionProvider: any NotificationPermissionProvider
  private let apiService: APINotificationService
  private let remoteRegistrar: any RemoteNotificationRegistering

  public init(
    permissionProvider: any NotificationPermissionProvider,
    apiService: APINotificationService,
    remoteRegistrar: any RemoteNotificationRegistering
  ) {
    self.permissionProvider = permissionProvider
    self.apiService = apiService
    self.remoteRegistrar = remoteRegistrar
  }

  public func requestPermission() async throws -> Bool {
    let granted = try await permissionProvider.requestPermission()
    if granted {
      remoteRegistrar.registerForRemoteNotifications()
    }
    return granted
  }

  public func registerDeviceToken(_ token: String) async throws {
    try await apiService.registerDeviceToken(token)
  }

  public func removeDeviceToken() async throws {
    try await apiService.removeDeviceToken()
  }
}
