import Foundation
import Testing
import TownCrierData
import TownCrierDomain

/// Verifies the `UserDefaults`-backed review-prompt store round-trips the full
/// state locally (GH #628). Mirrors `UserDefaultsOnboardingRepositoryTests`.
@Suite("UserDefaultsReviewPromptStore")
struct UserDefaultsReviewPromptStoreTests {
  private func makeSUT() throws -> (UserDefaultsReviewPromptStore, UserDefaults) {
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    return (UserDefaultsReviewPromptStore(defaults: defaults), defaults)
  }

  @Test("load returns a default-initialised state on first run")
  func loadsDefaultStateOnFirstRun() throws {
    let (sut, _) = try makeSUT()

    let state = sut.load()

    #expect(state == ReviewPromptState())
    #expect(state.firstLaunchDate == nil)
    #expect(state.engagementScore == 0)
    #expect(state.promptTimestamps.isEmpty)
    #expect(!state.hasRecordedUpgrade)
  }

  @Test("save then load round-trips the full state")
  func roundTripsFullState() throws {
    let (sut, _) = try makeSUT()
    let saved = ReviewPromptState(
      firstLaunchDate: Date(timeIntervalSince1970: 1_700_000_000),
      engagementScore: 5,
      saveCount: 3,
      lastActiveDayKey: "2023-11-14",
      distinctActiveDays: 4,
      lastPromptDate: Date(timeIntervalSince1970: 1_700_100_000),
      promptTimestamps: [
        Date(timeIntervalSince1970: 1_600_000_000),
        Date(timeIntervalSince1970: 1_700_100_000),
      ],
      hasRecordedUpgrade: true
    )

    sut.save(saved)

    #expect(sut.load() == saved)
  }

  @Test("state persists across store instances sharing the same defaults")
  func persistsAcrossInstances() throws {
    let (sut, defaults) = try makeSUT()
    let saved = ReviewPromptState(
      firstLaunchDate: Date(timeIntervalSince1970: 1_700_000_000),
      engagementScore: 6,
      hasRecordedUpgrade: true
    )

    sut.save(saved)
    let reopened = UserDefaultsReviewPromptStore(defaults: defaults)

    #expect(reopened.load() == saved)
  }

  @Test("optional dates persist as nil when unset")
  func optionalDatesNilWhenUnset() throws {
    let (sut, _) = try makeSUT()

    sut.save(ReviewPromptState(engagementScore: 2))

    let loaded = sut.load()
    #expect(loaded.firstLaunchDate == nil)
    #expect(loaded.lastPromptDate == nil)
    #expect(loaded.lastActiveDayKey == nil)
    #expect(loaded.engagementScore == 2)
  }
}
