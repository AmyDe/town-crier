/// Domain-level authorization status for system notification permission.
///
/// Maps `UNAuthorizationStatus` (`notDetermined`, `denied`, `authorized`,
/// `provisional`, `ephemeral`) into the three states the UI cares about.
/// `authorized` covers `provisional` and `ephemeral` because the user-facing
/// behaviour (notifications can be delivered) is identical.
public enum NotificationAuthorizationStatus: Sendable, Equatable {
  case notDetermined
  case denied
  case authorized
}
