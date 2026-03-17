import Foundation
import Testing
import TownCrierDomain

@Suite("WatchZoneStore")
struct WatchZoneStoreTests {
    @Test func spy_storeAndRetrieve_roundTrips() async throws {
        let spy = SpyWatchZoneStore()
        let zone = WatchZone.cambridge

        await spy.store([zone])
        let retrieved = await spy.retrieveAll()

        #expect(retrieved.count == 1)
        #expect(retrieved.first == zone)
        #expect(spy.storeCalls.count == 1)
    }

    @Test func spy_remove_deletesById() async throws {
        let spy = SpyWatchZoneStore()
        await spy.store([.cambridge, .london])

        await spy.remove(WatchZoneId("zone-001"))

        let remaining = await spy.retrieveAll()
        #expect(remaining.count == 1)
        #expect(remaining.first == .london)
        #expect(spy.removeCalls.count == 1)
    }
}

@Suite("NotificationStore")
struct NotificationStoreTests {
    @Test func spy_storeAndRetrieve_roundTrips() async throws {
        let spy = SpyNotificationStore()
        let record = NotificationRecord(
            id: "notif-1",
            applicationId: PlanningApplicationId("APP-001"),
            title: "Test",
            body: "Body",
            receivedAt: Date()
        )

        await spy.store(record)
        let all = await spy.retrieveAll()

        #expect(all.count == 1)
        #expect(all.first?.id == "notif-1")
        #expect(spy.storeSingleCalls.count == 1)
    }

    @Test func spy_markAsRead_updatesRecord() async throws {
        let spy = SpyNotificationStore()
        let record = NotificationRecord(
            id: "notif-1",
            applicationId: PlanningApplicationId("APP-001"),
            title: "Test",
            body: "Body",
            receivedAt: Date()
        )
        await spy.store(record)

        await spy.markAsRead("notif-1")
        let all = await spy.retrieveAll()

        #expect(all.first?.isRead == true)
    }
}
