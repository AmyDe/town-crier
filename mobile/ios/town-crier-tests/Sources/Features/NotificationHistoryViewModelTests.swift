import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("NotificationHistoryViewModel")
@MainActor
struct NotificationHistoryViewModelTests {
    private func makeSUT() -> (NotificationHistoryViewModel, SpyNotificationStore) {
        let spy = SpyNotificationStore()
        let vm = NotificationHistoryViewModel(store: spy)
        return (vm, spy)
    }

    @Test func loadNotifications_populatesSortedByDateDescending() async {
        let (sut, spy) = makeSUT()
        let older = NotificationRecord(
            id: "n1",
            applicationId: PlanningApplicationId("APP-001"),
            title: "Older",
            body: "Older body",
            receivedAt: Date(timeIntervalSince1970: 1_700_000_000)
        )
        let newer = NotificationRecord(
            id: "n2",
            applicationId: PlanningApplicationId("APP-002"),
            title: "Newer",
            body: "Newer body",
            receivedAt: Date(timeIntervalSince1970: 1_700_100_000)
        )
        await spy.store(older)
        await spy.store(newer)

        await sut.loadNotifications()

        #expect(sut.notifications.count == 2)
        #expect(sut.notifications.first?.id == "n2")
    }

    @Test func markAsRead_updatesLocalState() async {
        let (sut, spy) = makeSUT()
        let record = NotificationRecord(
            id: "n1",
            applicationId: PlanningApplicationId("APP-001"),
            title: "Test",
            body: "Body",
            receivedAt: Date()
        )
        await spy.store(record)
        await sut.loadNotifications()

        await sut.markAsRead("n1")

        #expect(sut.notifications.first?.isRead == true)
        #expect(spy.markAsReadCalls == ["n1"])
    }

    @Test func unreadCount_returnsCorrectCount() async {
        let (sut, spy) = makeSUT()
        let unread = NotificationRecord(
            id: "n1",
            applicationId: PlanningApplicationId("APP-001"),
            title: "Unread",
            body: "Body",
            receivedAt: Date()
        )
        let read = NotificationRecord(
            id: "n2",
            applicationId: PlanningApplicationId("APP-002"),
            title: "Read",
            body: "Body",
            receivedAt: Date(),
            isRead: true
        )
        await spy.store(unread)
        await spy.store(read)
        await sut.loadNotifications()

        #expect(sut.unreadCount == 1)
    }

    @Test func isEmpty_trueWhenNoNotifications() async {
        let (sut, _) = makeSUT()

        await sut.loadNotifications()

        #expect(sut.isEmpty)
    }
}
