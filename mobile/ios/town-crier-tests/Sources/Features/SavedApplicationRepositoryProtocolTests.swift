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

  @Test("save records the full PlanningApplication")
  func save_recordsApplication() async throws {
    let spy = SpySavedApplicationRepository()
    let app = PlanningApplication.pendingReview

    try await spy.save(application: app)

    #expect(spy.saveCalls.count == 1)
    #expect(spy.saveCalls[0].id == app.id)
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
