/// Fetches and updates per-zone notification preferences.
///
/// Maps to `GET/PUT /v1/me/watch-zones/{zoneId}/preferences`.
/// The `updatePreferences` call is entitlement-gated on the API side --
/// Personal+ is required for status change and decision update toggles.
public protocol ZonePreferencesRepository: Sendable {
  func fetchPreferences(zoneId: String) async throws -> ZoneNotificationPreferences
  func updatePreferences(_ preferences: ZoneNotificationPreferences) async throws
}
