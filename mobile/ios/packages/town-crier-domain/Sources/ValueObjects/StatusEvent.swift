import Foundation

/// A single status change in a planning application's lifecycle.
public struct StatusEvent: Equatable, Comparable, Sendable {
    public let status: ApplicationStatus
    public let date: Date

    public init(status: ApplicationStatus, date: Date) {
        self.status = status
        self.date = date
    }

    public static func < (lhs: StatusEvent, rhs: StatusEvent) -> Bool {
        lhs.date < rhs.date
    }
}
