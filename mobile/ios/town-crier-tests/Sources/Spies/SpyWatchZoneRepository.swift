import Foundation
import TownCrierDomain

final class SpyWatchZoneRepository: WatchZoneRepository, @unchecked Sendable {
    private(set) var saveCalls: [WatchZone] = []
    var saveResult: Result<Void, Error> = .success(())

    func save(_ zone: WatchZone) async throws {
        saveCalls.append(zone)
        try saveResult.get()
    }

    private(set) var loadActiveCallCount = 0
    var loadActiveResult: Result<WatchZone?, Error> = .success(nil)

    func loadActive() async throws -> WatchZone? {
        loadActiveCallCount += 1
        return try loadActiveResult.get()
    }
}
