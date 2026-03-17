/// Port for local persistence of watch zones.
public protocol WatchZoneStore: Sendable {
    func store(_ zones: [WatchZone]) async
    func retrieveAll() async -> [WatchZone]
    func remove(_ id: WatchZoneId) async
}
