import Foundation
import TownCrierDomain

final class SpyNotificationStore: NotificationStore, @unchecked Sendable {
    private(set) var storeSingleCalls: [NotificationRecord] = []
    private(set) var markAsReadCalls: [String] = []
    private var records: [NotificationRecord] = []

    func store(_ record: NotificationRecord) async {
        storeSingleCalls.append(record)
        records.append(record)
    }

    func retrieveAll() async -> [NotificationRecord] {
        records
    }

    func markAsRead(_ id: String) async {
        markAsReadCalls.append(id)
        if let index = records.firstIndex(where: { $0.id == id }) {
            records[index].markAsRead()
        }
    }
}
