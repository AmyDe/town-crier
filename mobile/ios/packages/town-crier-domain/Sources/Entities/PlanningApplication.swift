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
  /// `nil` when no notification exists strictly after the user's
  /// `lastReadAt` watermark — drives the muted styling of the row's status
  /// pill on the Applications screen.
  /// Spec: `docs/specs/notifications-unread-watermark.md#api-augment-applications`.
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
}
