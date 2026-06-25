import Foundation

/// The outcome of a single review-prompt evaluation: whether to ask, plus the
/// state to persist afterwards.
public enum ReviewPromptDecision: Equatable, Sendable {
  /// Present Apple's native review dialog now.
  case fire
  /// Do not present the dialog.
  case hold
}

/// The result of ``ReviewPromptPolicy/evaluate(signal:state:sessionSuppressed:isReactivation:)``:
/// the decision and the updated state the caller should persist.
public struct ReviewPromptOutcome: Equatable, Sendable {
  public let decision: ReviewPromptDecision
  public let state: ReviewPromptState

  public init(decision: ReviewPromptDecision, state: ReviewPromptState) {
    self.decision = decision
    self.state = state
  }
}

/// Pure, deterministic decision engine for the App Store review prompt (GH #628).
///
/// Given an incoming ``ReviewSignal``, the persisted ``ReviewPromptState``, a
/// transient session-suppression flag, and whether the current app foreground is
/// a background→active re-entry, it applies the signal's weight, enforces every
/// guard, and returns a ``ReviewPromptOutcome``. It performs no I/O and never
/// touches StoreKit, so it can be exhaustively unit-tested.
///
/// The scarce resource is Apple's ~3-prompts-per-year quota, so the policy only
/// fires at a genuine engagement peak (the score has reached the threshold *at*
/// a fire-eligible signal) and never after friction.
public struct ReviewPromptPolicy: Sendable {
  // MARK: - Tunable constants (the `upgradeWeight < engagementThreshold`
  // invariant must be preserved).

  /// The score at or above which a fire-eligible signal may fire.
  public static let engagementThreshold = 6
  public static let portalWeight = 3
  public static let savedApplicationWeight = 2
  public static let openedAlertWeight = 2
  public static let activeDayWeight = 1
  public static let upgradeWeight = 2

  /// Minimum account age before the first prompt (7 days).
  public static let minimumAccountAge: TimeInterval = 7 * 86_400
  /// Minimum gap between two prompts (120 days).
  public static let promptCooldown: TimeInterval = 120 * 86_400
  /// Rolling window for the belt-and-braces annual cap (365 days).
  public static let annualCapWindow: TimeInterval = 365 * 86_400
  /// Maximum prompt attempts allowed within ``annualCapWindow``.
  public static let annualCapLimit = 3
  /// Distinct active days required before a loyalty day becomes fire-eligible.
  public static let loyaltyDistinctDayThreshold = 3

  private let now: @Sendable () -> Date
  private let calendar: Calendar

  public init(now: @escaping @Sendable () -> Date = Date.init, calendar: Calendar = .current) {
    self.now = now
    self.calendar = calendar
  }

  /// Applies `signal` to `state`, enforces the guards, and returns the decision
  /// plus the state to persist. The signal's weight is always applied (so score
  /// accrues even when a guard holds the prompt); the score is reset only on a
  /// successful fire.
  public func evaluate(
    signal: ReviewSignal,
    state: ReviewPromptState,
    sessionSuppressed: Bool,
    isReactivation: Bool
  ) -> ReviewPromptOutcome {
    var newState = state
    let fireEligible = apply(signal, to: &newState, isReactivation: isReactivation)

    guard fireEligible, passesGuards(newState, sessionSuppressed: sessionSuppressed) else {
      return ReviewPromptOutcome(decision: .hold, state: newState)
    }

    let firedAt = now()
    newState.engagementScore = 0
    newState.lastPromptDate = firedAt
    newState.promptTimestamps.append(firedAt)
    return ReviewPromptOutcome(decision: .fire, state: newState)
  }

  // MARK: - Scoring

  /// Mutates `state` for the signal and returns whether the signal is a
  /// fire-eligible peak (a candidate for the guard checks).
  private func apply(
    _ signal: ReviewSignal,
    to state: inout ReviewPromptState,
    isReactivation: Bool
  ) -> Bool {
    switch signal {
    case .tappedPortal:
      state.engagementScore += Self.portalWeight
      return true

    case .openedAlert:
      state.engagementScore += Self.openedAlertWeight
      return true

    case .savedApplication:
      state.saveCount += 1
      // The first save is not counted; only the 2nd and later saves contribute.
      guard state.saveCount >= 2 else { return false }
      state.engagementScore += Self.savedApplicationWeight
      return true

    case .activeDay:
      let key = dayKey(for: now())
      // Never double-count the same calendar day.
      guard key != state.lastActiveDayKey else { return false }
      state.lastActiveDayKey = key
      state.distinctActiveDays += 1
      state.engagementScore += Self.activeDayWeight
      // Loyalty only becomes a fire moment once enough distinct days have
      // accrued, and only on a background→active re-entry (never a cold launch).
      return isReactivation && state.distinctActiveDays >= Self.loyaltyDistinctDayThreshold

    case .upgraded:
      // Latched: contributes at most once across the app's lifetime, and is
      // never itself a fire moment.
      if !state.hasRecordedUpgrade {
        state.engagementScore += Self.upgradeWeight
        state.hasRecordedUpgrade = true
      }
      return false
    }
  }

  // MARK: - Guards

  private func passesGuards(_ state: ReviewPromptState, sessionSuppressed: Bool) -> Bool {
    guard !sessionSuppressed else { return false }
    guard state.engagementScore >= Self.engagementThreshold else { return false }

    let current = now()

    // Account age.
    guard let firstLaunch = state.firstLaunchDate,
      current.timeIntervalSince(firstLaunch) >= Self.minimumAccountAge
    else { return false }

    // Cooldown. A backward clock (current < lastPromptDate) yields a negative
    // elapsed interval, which is < the cooldown, so it correctly holds.
    if let lastPrompt = state.lastPromptDate {
      guard current.timeIntervalSince(lastPrompt) >= Self.promptCooldown else { return false }
    }

    // Belt-and-braces annual cap.
    let windowStart = current.addingTimeInterval(-Self.annualCapWindow)
    let recentPrompts = state.promptTimestamps.filter { $0 >= windowStart }
    guard recentPrompts.count < Self.annualCapLimit else { return false }

    return true
  }

  // MARK: - Day key

  /// Derives a calendar-day key (`yyyy-M-d`) in the policy's calendar so distinct
  /// days are counted on day boundaries, never on time deltas — robust to
  /// timezone and backward-clock changes.
  private func dayKey(for date: Date) -> String {
    let components = calendar.dateComponents([.year, .month, .day], from: date)
    return "\(components.year ?? 0)-\(components.month ?? 0)-\(components.day ?? 0)"
  }
}
