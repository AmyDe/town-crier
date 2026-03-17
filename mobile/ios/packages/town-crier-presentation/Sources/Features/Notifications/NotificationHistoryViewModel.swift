import Combine
import TownCrierDomain

/// ViewModel for the notification history screen.
@MainActor
public final class NotificationHistoryViewModel: ObservableObject {
    @Published private(set) var notifications: [NotificationRecord] = []

    private let store: NotificationStore

    public var unreadCount: Int {
        notifications.filter { !$0.isRead }.count
    }

    public var isEmpty: Bool {
        notifications.isEmpty
    }

    public init(store: NotificationStore) {
        self.store = store
    }

    public func loadNotifications() async {
        let all = await store.retrieveAll()
        notifications = all.sorted { $0.receivedAt > $1.receivedAt }
    }

    public func markAsRead(_ id: String) async {
        await store.markAsRead(id)
        if let index = notifications.firstIndex(where: { $0.id == id }) {
            notifications[index].markAsRead()
        }
    }
}
