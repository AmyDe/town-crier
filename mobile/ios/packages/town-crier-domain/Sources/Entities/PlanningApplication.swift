import Foundation

/// A planning application submitted to a local authority.
public struct PlanningApplication: Equatable, Identifiable, Sendable {
  public let id: PlanningApplicationId
  public let reference: ApplicationReference
  public let authority: LocalAuthority
  public private(set) var status: ApplicationStatus
  public let receivedDate: Date
  public let description: String
  public let address: String
  public let location: Coordinate?
  public let portalUrl: URL?
  public let statusHistory: [StatusEvent]
  /// Per-row unread descriptor surfaced by the per-zone applications endpoint.
  /// `nil` when the application has no unread notification for the user
  /// (server-side `read_at IS NULL`) — drives the muted styling of the row's
  /// status pill on the Applications screen. See ADR 0035
  /// (`docs/adr/0035-per-application-notification-read-state.md`).
  public let latestUnreadEvent: LatestUnreadEvent?

  public init(
    id: PlanningApplicationId,
    reference: ApplicationReference,
    authority: LocalAuthority,
    status: ApplicationStatus,
    receivedDate: Date,
    description: String,
    address: String,
    location: Coordinate? = nil,
    portalUrl: URL? = nil,
    statusHistory: [StatusEvent] = [],
    latestUnreadEvent: LatestUnreadEvent? = nil
  ) {
    self.id = id
    self.reference = reference
    self.authority = authority
    self.status = status
    self.receivedDate = receivedDate
    self.description = description
    self.address = address
    self.location = location
    self.portalUrl = portalUrl
    self.statusHistory = statusHistory
    self.latestUnreadEvent = latestUnreadEvent
  }

  /// Returns a copy with `latestUnreadEvent` replaced. Used to optimistically
  /// clear a row's unread badge on tap-to-read (ADR 0035) without rebuilding
  /// every field, and by unread-UI tests to flip the read/unread bit.
  public func withLatestUnreadEvent(_ event: LatestUnreadEvent?) -> PlanningApplication {
    PlanningApplication(
      id: id,
      reference: reference,
      authority: authority,
      status: status,
      receivedDate: receivedDate,
      description: description,
      address: address,
      location: location,
      portalUrl: portalUrl,
      statusHistory: statusHistory,
      latestUnreadEvent: event
    )
  }
}
