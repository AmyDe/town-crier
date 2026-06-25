import Foundation
import Testing
import TownCrierDomain

/// Exhaustive unit tests for the pure ``ReviewPromptPolicy`` decision engine
/// (GH #628). Every weight, the threshold, and every guard is covered here with
/// a deterministic injected clock and a fixed-timezone calendar so distinct-day
/// counting is reproducible regardless of the host machine's locale.
@Suite("ReviewPromptPolicy")
struct ReviewPromptPolicyTests {
  // 2023-11-14 22:13:20 UTC — a fixed anchor for all time-dependent assertions.
  private let reference = Date(timeIntervalSince1970: 1_700_000_000)
  private let day: TimeInterval = 86_400

  private var utcCalendar: Calendar {
    var calendar = Calendar(identifier: .gregorian)
    calendar.timeZone = TimeZone(identifier: "UTC") ?? .current
    return calendar
  }

  private func makePolicy(now: Date) -> ReviewPromptPolicy {
    ReviewPromptPolicy(now: { now }, calendar: utcCalendar)
  }

  /// A state whose account is comfortably older than the 7-day age guard and
  /// that has never been prompted, so only the assertion under test gates a fire.
  private func eligibleState(
    engagementScore: Int = 0,
    saveCount: Int = 0,
    lastActiveDayKey: String? = nil,
    distinctActiveDays: Int = 0,
    lastPromptDate: Date? = nil,
    promptTimestamps: [Date] = [],
    hasRecordedUpgrade: Bool = false
  ) -> ReviewPromptState {
    ReviewPromptState(
      firstLaunchDate: reference.addingTimeInterval(-30 * day),
      engagementScore: engagementScore,
      saveCount: saveCount,
      lastActiveDayKey: lastActiveDayKey,
      distinctActiveDays: distinctActiveDays,
      lastPromptDate: lastPromptDate,
      promptTimestamps: promptTimestamps,
      hasRecordedUpgrade: hasRecordedUpgrade
    )
  }

  private func evaluate(
    _ signal: ReviewSignal,
    state: ReviewPromptState,
    now: Date? = nil,
    sessionSuppressed: Bool = false,
    isReactivation: Bool = false
  ) -> ReviewPromptOutcome {
    makePolicy(now: now ?? reference).evaluate(
      signal: signal,
      state: state,
      sessionSuppressed: sessionSuppressed,
      isReactivation: isReactivation
    )
  }

  // MARK: - Threshold

  @Test("holds when the accumulated score stays below the threshold")
  func holdsBelowThreshold() {
    let outcome = evaluate(.tappedPortal, state: eligibleState(engagementScore: 0))

    #expect(outcome.decision == .hold)
    #expect(outcome.state.engagementScore == 3)
  }

  @Test("fires when a fire-eligible signal lifts the score to exactly the threshold")
  func firesAtThreshold() {
    let outcome = evaluate(.tappedPortal, state: eligibleState(engagementScore: 3))

    #expect(outcome.decision == .fire)
  }

  // MARK: - Weights

  @Test("tapping through to the portal contributes +3")
  func portalWeight() {
    let outcome = evaluate(.tappedPortal, state: eligibleState(engagementScore: 0))
    #expect(outcome.state.engagementScore == ReviewPromptPolicy.portalWeight)
    #expect(ReviewPromptPolicy.portalWeight == 3)
  }

  @Test("opening an instant alert contributes +2")
  func openedAlertWeight() {
    let outcome = evaluate(.openedAlert, state: eligibleState(engagementScore: 0))
    #expect(outcome.state.engagementScore == ReviewPromptPolicy.openedAlertWeight)
    #expect(ReviewPromptPolicy.openedAlertWeight == 2)
  }

  @Test("a loyalty active day contributes +1")
  func activeDayWeight() {
    let outcome = evaluate(
      .activeDay,
      state: eligibleState(engagementScore: 0, lastActiveDayKey: nil),
      isReactivation: true
    )
    #expect(outcome.state.engagementScore == ReviewPromptPolicy.activeDayWeight)
    #expect(ReviewPromptPolicy.activeDayWeight == 1)
  }

  @Test("portal (+3), a 2nd save (+2) and a loyalty day (+1) reach 6 and fire")
  func combinedSignalsReachThreshold() {
    // Pre-load portal (3) + 2nd save (2) = 5 with two prior distinct days, then
    // the third distinct day's loyalty point (+1) crosses the threshold.
    let state = eligibleState(
      engagementScore: 5,
      saveCount: 2,
      lastActiveDayKey: "2023-11-14",
      distinctActiveDays: 2
    )

    let outcome = evaluate(
      .activeDay,
      state: state,
      now: reference.addingTimeInterval(day),
      isReactivation: true
    )

    #expect(outcome.decision == .fire)
  }

  // MARK: - Upgrade — score contributor only, latched

  @Test("upgrading contributes +2 but never fires (weight < threshold invariant)")
  func upgradeNeverFires() {
    #expect(ReviewPromptPolicy.upgradeWeight < ReviewPromptPolicy.engagementThreshold)

    let outcome = evaluate(.upgraded, state: eligibleState(engagementScore: 0))

    #expect(outcome.decision == .hold)
    #expect(outcome.state.engagementScore == 2)
  }

  @Test("upgrade is latched — recording it twice still only adds +2 once")
  func upgradeIsLatched() {
    let first = evaluate(.upgraded, state: eligibleState(engagementScore: 0))
    #expect(first.state.engagementScore == 2)
    #expect(first.state.hasRecordedUpgrade)

    let second = evaluate(.upgraded, state: first.state)
    #expect(second.decision == .hold)
    #expect(second.state.engagementScore == 2)
  }

  // MARK: - Saves — first save not counted

  @Test("the first save increments the count but contributes no score and never fires")
  func firstSaveNotCounted() {
    let outcome = evaluate(.savedApplication, state: eligibleState(engagementScore: 5, saveCount: 0))

    #expect(outcome.decision == .hold)
    #expect(outcome.state.saveCount == 1)
    #expect(outcome.state.engagementScore == 5)
  }

  @Test("the second save contributes +2 and is fire-eligible")
  func secondSaveCountsAndCanFire() {
    let outcome = evaluate(.savedApplication, state: eligibleState(engagementScore: 4, saveCount: 1))

    #expect(outcome.decision == .fire)
    #expect(outcome.state.saveCount == 2)
  }

  // MARK: - Account-age guard

  @Test("never fires before the account is 7 days old, regardless of score")
  func accountAgeGuardBlocks() {
    let state = ReviewPromptState(
      firstLaunchDate: reference.addingTimeInterval(-6 * day),
      engagementScore: 3
    )

    let outcome = evaluate(.tappedPortal, state: state)

    #expect(outcome.decision == .hold)
    #expect(outcome.state.engagementScore == 6)  // score still accrues
  }

  @Test("fires once the account reaches 7 days old")
  func accountAgeGuardPasses() {
    let state = ReviewPromptState(
      firstLaunchDate: reference.addingTimeInterval(-7 * day),
      engagementScore: 3
    )

    let outcome = evaluate(.tappedPortal, state: state)

    #expect(outcome.decision == .fire)
  }

  @Test("never fires when the first-launch date is unknown")
  func accountAgeGuardBlocksWhenUnknown() {
    let state = ReviewPromptState(firstLaunchDate: nil, engagementScore: 3)

    let outcome = evaluate(.tappedPortal, state: state)

    #expect(outcome.decision == .hold)
  }

  // MARK: - Cooldown guard

  @Test("never fires within 120 days of the last prompt")
  func cooldownGuardBlocks() {
    let state = eligibleState(
      engagementScore: 3,
      lastPromptDate: reference.addingTimeInterval(-119 * day)
    )

    let outcome = evaluate(.tappedPortal, state: state)

    #expect(outcome.decision == .hold)
  }

  @Test("fires once 120 days have elapsed since the last prompt")
  func cooldownGuardPasses() {
    let state = eligibleState(
      engagementScore: 3,
      lastPromptDate: reference.addingTimeInterval(-120 * day)
    )

    let outcome = evaluate(.tappedPortal, state: state)

    #expect(outcome.decision == .fire)
  }

  @Test("holds when the clock has moved backwards past the last prompt date")
  func cooldownGuardHoldsOnBackwardClock() {
    let state = eligibleState(
      engagementScore: 3,
      lastPromptDate: reference.addingTimeInterval(10 * day)  // in the future
    )

    let outcome = evaluate(.tappedPortal, state: state)

    #expect(outcome.decision == .hold)
  }

  // MARK: - Annual cap guard

  @Test("never fires when 3 prompts already fall within the trailing 365 days")
  func annualCapGuardBlocks() {
    let state = eligibleState(
      engagementScore: 3,
      lastPromptDate: reference.addingTimeInterval(-200 * day),
      promptTimestamps: [
        reference.addingTimeInterval(-300 * day),
        reference.addingTimeInterval(-200 * day),
        reference.addingTimeInterval(-130 * day),
      ]
    )

    let outcome = evaluate(.tappedPortal, state: state)

    #expect(outcome.decision == .hold)
  }

  @Test("fires once the oldest of three prompts ages out past 365 days")
  func annualCapGuardUnblocksAsOldestAgesOut() {
    let state = eligibleState(
      engagementScore: 3,
      lastPromptDate: reference.addingTimeInterval(-200 * day),
      promptTimestamps: [
        reference.addingTimeInterval(-366 * day),  // aged out
        reference.addingTimeInterval(-300 * day),
        reference.addingTimeInterval(-200 * day),
      ]
    )

    let outcome = evaluate(.tappedPortal, state: state)

    #expect(outcome.decision == .fire)
  }

  // MARK: - Session suppression guard

  @Test("never fires while the session is suppressed")
  func sessionSuppressionGuardBlocks() {
    let outcome = evaluate(
      .tappedPortal,
      state: eligibleState(engagementScore: 3),
      sessionSuppressed: true
    )

    #expect(outcome.decision == .hold)
    #expect(outcome.state.engagementScore == 6)  // suppression does not stop accrual
  }

  // MARK: - Reset after fire

  @Test("after a fire the score resets and the prompt is timestamped")
  func resetsAfterFire() {
    let state = eligibleState(engagementScore: 3, promptTimestamps: [])

    let outcome = evaluate(.tappedPortal, state: state)

    #expect(outcome.decision == .fire)
    #expect(outcome.state.engagementScore == 0)
    #expect(outcome.state.lastPromptDate == reference)
    #expect(outcome.state.promptTimestamps == [reference])
  }

  // MARK: - Loyalty active-day eligibility

  @Test("loyalty is not fire-eligible before 3 distinct active days")
  func loyaltyNotEligibleBelowThreeDays() {
    let state = eligibleState(
      engagementScore: 8,  // already past the threshold
      lastActiveDayKey: "2023-11-14",
      distinctActiveDays: 1
    )

    let outcome = evaluate(
      .activeDay,
      state: state,
      now: reference.addingTimeInterval(day),
      isReactivation: true
    )

    #expect(outcome.decision == .hold)
    #expect(outcome.state.distinctActiveDays == 2)
  }

  @Test("loyalty is fire-eligible on the 3rd distinct day during a re-entry")
  func loyaltyEligibleAtThreeDaysOnReactivation() {
    let state = eligibleState(
      engagementScore: 5,
      lastActiveDayKey: "2023-11-14",
      distinctActiveDays: 2
    )

    let outcome = evaluate(
      .activeDay,
      state: state,
      now: reference.addingTimeInterval(day),
      isReactivation: true
    )

    #expect(outcome.decision == .fire)
    #expect(outcome.state.distinctActiveDays == 3)
  }

  @Test("loyalty never fires on a cold launch even at 3 distinct days")
  func loyaltyNotEligibleOnColdLaunch() {
    let state = eligibleState(
      engagementScore: 5,
      lastActiveDayKey: "2023-11-14",
      distinctActiveDays: 2
    )

    let outcome = evaluate(
      .activeDay,
      state: state,
      now: reference.addingTimeInterval(day),
      isReactivation: false
    )

    #expect(outcome.decision == .hold)
    #expect(outcome.state.distinctActiveDays == 3)
    #expect(outcome.state.engagementScore == 6)
  }

  @Test("the same calendar day is never counted twice")
  func sameDayNotDoubleCounted() {
    let state = eligibleState(
      engagementScore: 4,
      lastActiveDayKey: "2023-11-14",
      distinctActiveDays: 2
    )

    let outcome = evaluate(.activeDay, state: state, now: reference, isReactivation: true)

    #expect(outcome.decision == .hold)
    #expect(outcome.state.distinctActiveDays == 2)
    #expect(outcome.state.engagementScore == 4)
  }

  @Test("a backward clock change counts a new day without corrupting state")
  func backwardClockDoesNotCorruptDayCount() {
    let state = eligibleState(
      engagementScore: 0,
      lastActiveDayKey: "2023-11-15",
      distinctActiveDays: 3
    )

    // now is a day earlier than the last recorded key.
    let outcome = evaluate(.activeDay, state: state, now: reference, isReactivation: true)

    #expect(outcome.state.distinctActiveDays == 4)
    #expect(outcome.state.engagementScore == 1)
    #expect(outcome.state.lastActiveDayKey == "2023-11-14")
  }
}
