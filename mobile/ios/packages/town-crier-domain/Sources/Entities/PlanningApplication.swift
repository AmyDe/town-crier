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
    statusHistory: [StatusEvent] = []
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
  }

  public mutating func markAsDecided(_ decision: Decision, on decisionDate: Date) throws {
    let decidedStatus: ApplicationStatus
    switch decision {
    case .permitted:
      decidedStatus = .permitted
    case .conditions:
      decidedStatus = .conditions
    case .rejected:
      decidedStatus = .rejected
    }
    guard status == .undecided else {
      throw DomainError.invalidStatusTransition(from: status, to: decidedStatus)
    }
    status = decidedStatus
  }
}
