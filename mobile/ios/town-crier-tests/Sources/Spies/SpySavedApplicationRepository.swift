import TownCrierDomain

final class SpySavedApplicationRepository: SavedApplicationRepository, @unchecked Sendable {
  private(set) var saveCalls: [PlanningApplication] = []
  var saveResult: Result<Void, Error> = .success(())

  func save(application: PlanningApplication) async throws {
    saveCalls.append(application)
    try saveResult.get()
  }

  private(set) var removeCalls: [String] = []
  var removeResult: Result<Void, Error> = .success(())

  func remove(applicationUid: String) async throws {
    removeCalls.append(applicationUid)
    try removeResult.get()
  }

  private(set) var loadAllCallCount = 0
  var loadAllResult: Result<[SavedApplication], Error> = .success([])

  func loadAll() async throws -> [SavedApplication] {
    loadAllCallCount += 1
    return try loadAllResult.get()
  }
}
