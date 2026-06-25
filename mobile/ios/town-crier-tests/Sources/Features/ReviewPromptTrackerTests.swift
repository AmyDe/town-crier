import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests the thin @MainActor ``ReviewPromptTracker`` service that the app talks
/// to (GH #628): it updates the store, evaluates the policy, requests a review
/// on a fire, and honours session suppression.
@Suite("ReviewPromptTracker")
@MainActor
struct ReviewPromptTrackerTests {
  private let reference = Date(timeIntervalSince1970: 1_700_000_000)
  private let day: TimeInterval = 86_400

  /// A store seeded so the account is older than the age guard and only the
  /// behaviour under test gates a fire.
  private func makeStore(
    engagementScore: Int = 0,
    saveCount: Int = 0,
    distinctActiveDays: Int = 0,
    lastActiveDayKey: String? = nil
  ) -> FakeReviewPromptStore {
    FakeReviewPromptStore(
      state: ReviewPromptState(
        firstLaunchDate: reference.addingTimeInterval(-30 * day),
        engagementScore: engagementScore,
        saveCount: saveCount,
        lastActiveDayKey: lastActiveDayKey,
        distinctActiveDays: distinctActiveDays
      )
    )
  }

  private func makeTracker(
    store: FakeReviewPromptStore,
    requester: SpyReviewRequester
  ) -> ReviewPromptTracker {
    let now = reference
    return ReviewPromptTracker(store: store, requester: requester) { now }
  }

  @Test("record applies the signal weight and persists it")
  func recordUpdatesStore() {
    let store = makeStore(engagementScore: 0)
    let tracker = makeTracker(store: store, requester: SpyReviewRequester())

    tracker.record(.tappedPortal)

    #expect(store.load().engagementScore == 3)
  }

  @Test("a fire-eligible signal that crosses the threshold requests a review and resets")
  func fireRequestsReviewAndResets() {
    let store = makeStore(engagementScore: 4)
    let requester = SpyReviewRequester()
    let tracker = makeTracker(store: store, requester: requester)

    tracker.record(.openedAlert)  // +2 -> 6

    #expect(requester.requestReviewCallCount == 1)
    #expect(store.load().engagementScore == 0)
  }

  @Test("a below-threshold signal does not request a review")
  func belowThresholdDoesNotRequest() {
    let store = makeStore(engagementScore: 0)
    let requester = SpyReviewRequester()
    let tracker = makeTracker(store: store, requester: requester)

    tracker.record(.tappedPortal)  // +3 -> 3

    #expect(requester.requestReviewCallCount == 0)
  }

  @Test("upgrading never requests a review even when repeated")
  func upgradeNeverRequests() {
    let store = makeStore(engagementScore: 5)
    let requester = SpyReviewRequester()
    let tracker = makeTracker(store: store, requester: requester)

    tracker.record(.upgraded)  // +2 -> 7 but not fire-eligible
    tracker.record(.upgraded)  // latched, no change

    #expect(requester.requestReviewCallCount == 0)
    #expect(store.load().engagementScore == 7)
  }

  @Test("suppressThisSession holds a subsequent fire-eligible signal")
  func suppressHoldsFire() {
    let store = makeStore(engagementScore: 4)
    let requester = SpyReviewRequester()
    let tracker = makeTracker(store: store, requester: requester)

    tracker.suppressThisSession()
    tracker.record(.openedAlert)  // would reach 6 but session is suppressed

    #expect(requester.requestReviewCallCount == 0)
    #expect(store.load().engagementScore == 6)  // accrual still happens
  }

  @Test("recordAppForegrounded records a loyalty active day")
  func foregroundRecordsActiveDay() {
    let store = makeStore(distinctActiveDays: 0, lastActiveDayKey: nil)
    let tracker = makeTracker(store: store, requester: SpyReviewRequester())

    tracker.recordAppForegrounded(isReactivation: true)

    #expect(store.load().distinctActiveDays == 1)
    #expect(store.load().lastActiveDayKey != nil)
  }

  @Test("the first-launch date is established on init when absent")
  func firstLaunchEstablishedOnInit() {
    let store = FakeReviewPromptStore(state: ReviewPromptState(firstLaunchDate: nil))
    let requester = SpyReviewRequester()
    let now = reference

    _ = ReviewPromptTracker(store: store, requester: requester) { now }

    #expect(store.load().firstLaunchDate == reference)
  }

  @Test("an existing first-launch date is not overwritten on init")
  func firstLaunchNotOverwritten() {
    let original = reference.addingTimeInterval(-100 * day)
    let store = FakeReviewPromptStore(state: ReviewPromptState(firstLaunchDate: original))
    let now = reference

    _ = ReviewPromptTracker(store: store, requester: SpyReviewRequester()) { now }

    #expect(store.load().firstLaunchDate == original)
  }

  @Test("back-to-back signals each accrue, firing exactly once on the crossing")
  func backToBackSignalsEvaluatedSerially() {
    let store = makeStore(engagementScore: 0)
    let requester = SpyReviewRequester()
    let tracker = makeTracker(store: store, requester: requester)

    tracker.record(.openedAlert)  // 2
    tracker.record(.openedAlert)  // 4
    tracker.record(.openedAlert)  // 6 -> fire, reset to 0

    #expect(requester.requestReviewCallCount == 1)
    #expect(store.load().engagementScore == 0)
  }
}
