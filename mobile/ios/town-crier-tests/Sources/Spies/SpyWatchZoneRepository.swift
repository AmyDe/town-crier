import Foundation
import TownCrierDomain

final class SpyWatchZoneRepository: WatchZoneRepository, @unchecked Sendable {
  private(set) var saveCalls: [WatchZone] = []
  var saveResult: Result<Void, Error> = .success(())

  func save(_ zone: WatchZone) async throws {
    saveCalls.append(zone)
    try saveResult.get()
  }

  private(set) var loadAllCallCount = 0
  var loadAllResult: Result<[WatchZone], Error> = .success([])

  func loadAll() async throws -> [WatchZone] {
    loadAllCallCount += 1
    return try loadAllResult.get()
  }

  private(set) var deleteCalls: [WatchZoneId] = []
  var deleteResult: Result<Void, Error> = .success(())

  func delete(_ id: WatchZoneId) async throws {
    deleteCalls.append(id)
    try deleteResult.get()
  }
}
