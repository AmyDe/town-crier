import Foundation

/// A positive in-app engagement signal that feeds the App Store review-prompt
/// policy (GH #628).
///
/// Signals are deliberately scoped to *value* moments inside the app — a review
/// ask is never delivered by push. Most signals can trigger a fire evaluation;
/// ``upgraded`` is a pure score contributor that nudges a borderline user toward
/// a prompt but can never, on its own, be the moment that triggers the ask.
public enum ReviewSignal: Equatable, Sendable {
  /// The user tapped through to the council planning portal.
  case tappedPortal
  /// The user saved/bookmarked an application. Only the 2nd and later saves are
  /// fire-eligible; the first save is usually setup rather than delight.
  case savedApplication
  /// The user opened an instant alert via its push/deep-link detail path. This
  /// is the alert-payoff moment and is distinct from browsing the list.
  case openedAlert
  /// The app was foregrounded on a new distinct calendar day (loyalty).
  case activeDay
  /// The user upgraded to a paid tier. Contributes score but is never a fire
  /// moment and is latched so it counts at most once.
  case upgraded

  /// Whether this signal can ever be the moment that triggers a review ask.
  ///
  /// ``upgraded`` is excluded outright; the remaining signals are *candidate*
  /// fire moments whose final eligibility is further qualified by the policy
  /// (e.g. a save is only eligible on the 2nd+ occurrence, a loyalty day only
  /// once enough distinct days have accrued and only on a re-entry).
  public var canTriggerFire: Bool {
    switch self {
    case .upgraded:
      return false
    case .tappedPortal, .savedApplication, .openedAlert, .activeDay:
      return true
    }
  }
}

/// The locally-persisted state the review-prompt policy reads and updates.
///
/// Every field is device-local: there is no server telemetry, analytics, or PII
/// involved in deciding when to ask for a rating.
public struct ReviewPromptState: Equatable, Sendable {
  /// When this device first ran the app with the review feature — the anchor for
  /// the account-age guard. `nil` until the tracker establishes it on first run.
  public var firstLaunchDate: Date?
  /// The accumulated engagement score, reset to 0 after a fire.
  public var engagementScore: Int
  /// How many applications the user has saved (gates the first-save exclusion).
  public var saveCount: Int
  /// The `yyyy-M-d` key of the most recent active day, so the same calendar day
  /// is never double-counted.
  public var lastActiveDayKey: String?
  /// The number of distinct calendar days the app has been foregrounded on.
  public var distinctActiveDays: Int
  /// When the last review prompt was attempted, anchoring the cooldown guard.
  public var lastPromptDate: Date?
  /// Timestamps of past prompt attempts, used for the rolling annual cap.
  public var promptTimestamps: [Date]
  /// Whether the upgrade score contribution has already been latched.
  public var hasRecordedUpgrade: Bool

  public init(
    firstLaunchDate: Date? = nil,
    engagementScore: Int = 0,
    saveCount: Int = 0,
    lastActiveDayKey: String? = nil,
    distinctActiveDays: Int = 0,
    lastPromptDate: Date? = nil,
    promptTimestamps: [Date] = [],
    hasRecordedUpgrade: Bool = false
  ) {
    self.firstLaunchDate = firstLaunchDate
    self.engagementScore = engagementScore
    self.saveCount = saveCount
    self.lastActiveDayKey = lastActiveDayKey
    self.distinctActiveDays = distinctActiveDays
    self.lastPromptDate = lastPromptDate
    self.promptTimestamps = promptTimestamps
    self.hasRecordedUpgrade = hasRecordedUpgrade
  }
}
