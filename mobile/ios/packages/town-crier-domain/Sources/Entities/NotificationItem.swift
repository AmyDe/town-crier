import Foundation

/// A notification about a planning application event delivered to the user.
///
/// `eventType` carries the API's event taxonomy (e.g. `"NewApplication"`,
/// `"DecisionUpdate"`) so the presentation layer can decide whether to render
/// a decision badge. `decision` is the raw PlanIt `app_state` (e.g.
/// `"Permitted"`, `"Conditions"`, `"Rejected"`, `"Appealed"`) — `nil` for
/// non-decision events. `sources` is a flag string (e.g. `"Zone, Saved"`)
/// describing which user signal(s) led to this notification being delivered.
public struct NotificationItem: Equatable, Sendable {
  public let applicationName: String
  public let applicationAddress: String
  public let applicationDescription: String
  public let applicationType: String
  public let authorityId: Int
  public let createdAt: Date
  public let eventType: String
  public let decision: String?
  public let sources: String

  public init(
    applicationName: String,
    applicationAddress: String,
    applicationDescription: String,
    applicationType: String,
    authorityId: Int,
    createdAt: Date,
    eventType: String,
    decision: String?,
    sources: String
  ) {
    self.applicationName = applicationName
    self.applicationAddress = applicationAddress
    self.applicationDescription = applicationDescription
    self.applicationType = applicationType
    self.authorityId = authorityId
    self.createdAt = createdAt
    self.eventType = eventType
    self.decision = decision
    self.sources = sources
  }
}
