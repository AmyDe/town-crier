import Foundation

/// A record of a push notification received about a planning application.
public struct NotificationRecord: Equatable, Identifiable, Sendable {
    public let id: String
    public let applicationId: PlanningApplicationId
    public let title: String
    public let body: String
    public let receivedAt: Date
    public private(set) var isRead: Bool

    public init(
        id: String,
        applicationId: PlanningApplicationId,
        title: String,
        body: String,
        receivedAt: Date,
        isRead: Bool = false
    ) {
        self.id = id
        self.applicationId = applicationId
        self.title = title
        self.body = body
        self.receivedAt = receivedAt
        self.isRead = isRead
    }

    public mutating func markAsRead() {
        isRead = true
    }
}
