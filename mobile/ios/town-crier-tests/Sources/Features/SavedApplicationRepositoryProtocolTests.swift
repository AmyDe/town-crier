import Foundation
import Testing
import TownCrierDomain

@Suite("SavedApplicationRepository protocol")
struct SavedApplicationRepositoryProtocolTests {

  @Test("spy conforms to SavedApplicationRepository")
  func spy_conformsToProtocol() {
    let spy: any SavedApplicationRepository = SpySavedApplicationRepository()
    #expect(spy is SpySavedApplicationRepository)
  }

  @Test("save records the applicationUid")
  func save_recordsUid() async throws {
    let spy = SpySavedApplicationRepository()

    try await spy.save(applicationUid: "BK/2026/0042")

    #expect(spy.saveCalls == ["BK/2026/0042"])
  }

  @Test("remove records the applicationUid")
  func remove_recordsUid() async throws {
    let spy = SpySavedApplicationRepository()

    try await spy.remove(applicationUid: "BK/2026/0042")

    #expect(spy.removeCalls == ["BK/2026/0042"])
  }

  @Test("loadAll returns preconfigured result")
  func loadAll_returnsPreconfiguredResult() async throws {
    let spy = SpySavedApplicationRepository()
    let expected = [
      SavedApplication(
        applicationUid: "UID-1",
        savedAt: Date(timeIntervalSince1970: 1_700_000_000)
      )
    ]
    spy.loadAllResult = .success(expected)

    let result = try await spy.loadAll()

    #expect(result == expected)
  }
}
