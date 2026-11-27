import Foundation
import TownCrierDomain

final class SpyWatchZoneRepository: WatchZoneRepository, @unchecked Sendable {
  private(set) var saveCalls: [WatchZone] = []
  var saveResult: Result<Void, Error> = .success(())
  /// Per-call results consumed in order, one per `save(_:)` invocation —
  /// lets a test script a sequence (e.g. "first zone succeeds, second hits
  /// the quota"). Falls back to `saveResult` once exhausted (or if never
  /// set), so existing single-result tests are unaffected.
  var saveResults: [Result<Void, Error>] = []

  func save(_ zone: WatchZone) async throws {
    saveCalls.append(zone)
    if saveCalls.count <= saveResults.count {
      try saveResults[saveCalls.count - 1].get()
    } else {
      try saveResult.get()
    }
  }

  private(set) var updateCalls: [WatchZone] = []
  var updateResult: Result<Void, Error> = .success(())

  func update(_ zone: WatchZone) async throws {
    updateCalls.append(zone)
    try updateResult.get()
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
