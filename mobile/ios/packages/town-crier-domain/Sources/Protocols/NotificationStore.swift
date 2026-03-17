/// Port for local persistence of notification history.
public protocol NotificationStore: Sendable {
    func store(_ record: NotificationRecord) async
    func retrieveAll() async -> [NotificationRecord]
    func markAsRead(_ id: String) async
}
