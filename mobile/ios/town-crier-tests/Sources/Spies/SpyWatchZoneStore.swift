import TownCrierDomain

final class SpyWatchZoneStore: WatchZoneStore, @unchecked Sendable {
    private(set) var storeCalls: [[WatchZone]] = []
    private var zones: [WatchZone] = []

    func store(_ zones: [WatchZone]) async {
        storeCalls.append(zones)
        self.zones = zones
    }

    func retrieveAll() async -> [WatchZone] {
        zones
    }
}
