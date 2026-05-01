/// Per-zone notification preferences controlling which alert channels the user receives
/// for a specific watch zone.
///
/// The API shape matches `GET/PUT /v1/me/watch-zones/{zoneId}/preferences` (see
/// `GetZonePreferencesResult` / `UpdateZonePreferencesCommand` on the API). The four
/// per-channel toggles default to true so newly-created zones opt in to all alerts;
/// free-tier downgrades are applied at dispatch time on the server.
public struct ZoneNotificationPreferences: Equatable, Sendable {
  public let zoneId: String
  public let newApplicationPush: Bool
  public let newApplicationEmail: Bool
  public let decisionPush: Bool
  public let decisionEmail: Bool

  public init(
    zoneId: String,
    newApplicationPush: Bool = true,
    newApplicationEmail: Bool = true,
    decisionPush: Bool = true,
    decisionEmail: Bool = true
  ) {
    self.zoneId = zoneId
    self.newApplicationPush = newApplicationPush
    self.newApplicationEmail = newApplicationEmail
    self.decisionPush = decisionPush
    self.decisionEmail = decisionEmail
  }
}
