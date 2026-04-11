/// Per-zone notification preferences controlling which alert types the user receives
/// for a specific watch zone.
///
/// The API shape matches `GET/PUT /v1/me/watch-zones/{zoneId}/preferences`.
/// `statusChanges` and `decisionUpdates` are entitlement-gated (Personal+ only).
public struct ZoneNotificationPreferences: Equatable, Sendable {
  public let zoneId: String
  public let newApplications: Bool
  public let statusChanges: Bool
  public let decisionUpdates: Bool

  public init(
    zoneId: String,
    newApplications: Bool = true,
    statusChanges: Bool = false,
    decisionUpdates: Bool = false
  ) {
    self.zoneId = zoneId
    self.newApplications = newApplications
    self.statusChanges = statusChanges
    self.decisionUpdates = decisionUpdates
  }
}
