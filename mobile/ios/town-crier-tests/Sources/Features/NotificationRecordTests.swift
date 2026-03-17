import Foundation
import Testing
import TownCrierDomain

@Suite("NotificationRecord")
struct NotificationRecordTests {
    @Test func init_setsAllProperties() throws {
        let now = Date()
        let record = NotificationRecord(
            id: "notif-1",
            applicationId: PlanningApplicationId("APP-001"),
            title: "New application nearby",
            body: "Erection of two-storey rear extension at 12 Mill Road",
            receivedAt: now,
            isRead: false
        )

        #expect(record.id == "notif-1")
        #expect(record.applicationId == PlanningApplicationId("APP-001"))
        #expect(record.title == "New application nearby")
        #expect(record.body == "Erection of two-storey rear extension at 12 Mill Road")
        #expect(record.receivedAt == now)
        #expect(!record.isRead)
    }

    @Test func markAsRead_setsIsReadTrue() {
        let now = Date()
        var record = NotificationRecord(
            id: "notif-1",
            applicationId: PlanningApplicationId("APP-001"),
            title: "New application nearby",
            body: "A new planning application has been submitted",
            receivedAt: now,
            isRead: false
        )

        record.markAsRead()

        #expect(record.isRead)
    }

    @Test func isEquatable() {
        let now = Date()
        let a = NotificationRecord(
            id: "notif-1",
            applicationId: PlanningApplicationId("APP-001"),
            title: "Title",
            body: "Body",
            receivedAt: now,
            isRead: false
        )
        let b = NotificationRecord(
            id: "notif-1",
            applicationId: PlanningApplicationId("APP-001"),
            title: "Title",
            body: "Body",
            receivedAt: now,
            isRead: false
        )

        #expect(a == b)
    }
}
