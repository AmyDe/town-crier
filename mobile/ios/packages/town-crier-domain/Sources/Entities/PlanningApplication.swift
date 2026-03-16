import Foundation

/// A planning application submitted to a local authority.
public struct PlanningApplication: Equatable, Sendable {
    public let id: PlanningApplicationId
    public let reference: ApplicationReference
    public let authority: LocalAuthority
    public private(set) var status: ApplicationStatus
    public let receivedDate: Date
    public let description: String
    public let address: String

    public init(
        id: PlanningApplicationId,
        reference: ApplicationReference,
        authority: LocalAuthority,
        status: ApplicationStatus,
        receivedDate: Date,
        description: String,
        address: String
    ) {
        self.id = id
        self.reference = reference
        self.authority = authority
        self.status = status
        self.receivedDate = receivedDate
        self.description = description
        self.address = address
    }

    public mutating func markAsDecided(_ decision: Decision, on decisionDate: Date) throws {
        guard status == .underReview else {
            throw DomainError.invalidStatusTransition(
                from: status,
                to: decision == .approved ? .approved : .refused
            )
        }
        status = decision == .approved ? .approved : .refused
    }
}
