import Foundation
import TownCrierDomain
import os

/// Coordinates APNs device token registration with the backend.
///
/// Owns the queue/flush behaviour required when a device token arrives before
/// the user has authenticated: tokens are held until `flushPendingRegistration()`
/// is called (typically after a successful login). Once authenticated, tokens
/// are forwarded to `NotificationService.registerDeviceToken(_:)` immediately.
public actor PushNotificationRegistrar {
  private static let logger = Logger(
    subsystem: "uk.towncrierapp", category: "PushNotificationRegistrar"
  )

  private let notificationService: any NotificationService
  private let authService: any AuthenticationService
  private var queuedToken: String?

  public init(
    notificationService: any NotificationService,
    authService: any AuthenticationService
  ) {
    self.notificationService = notificationService
    self.authService = authService
  }

  /// Called by the AppDelegate when APNs returns a device token.
  /// Converts the token data to lowercased hex and either registers it
  /// immediately (if authenticated) or queues it for later flush.
  public func didReceiveDeviceToken(_ tokenData: Data) async {
    let hexToken = tokenData.hexEncodedString()
    if await authService.currentSession() != nil {
      await register(hexToken)
    } else {
      queuedToken = hexToken
    }
  }

  /// Flushes a queued device token to the backend, if one was captured before
  /// the user authenticated. Should be called after a successful login.
  public func flushPendingRegistration() async {
    guard let token = queuedToken else { return }
    guard await authService.currentSession() != nil else { return }
    await register(token)
    queuedToken = nil
  }

  /// Called by the AppDelegate when APNs registration fails. Logged but not
  /// surfaced — the app remains usable without push notifications.
  public func didFailToRegister(error: Error) {
    Self.logger.error(
      "APNs registration failed: \(error.localizedDescription)"
    )
  }

  private func register(_ hexToken: String) async {
    do {
      try await notificationService.registerDeviceToken(hexToken)
    } catch {
      Self.logger.error(
        "Failed to register APNs device token with backend: \(error.localizedDescription)"
      )
    }
  }
}
