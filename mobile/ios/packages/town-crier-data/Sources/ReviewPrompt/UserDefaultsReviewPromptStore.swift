import Foundation
import TownCrierDomain

/// Persists ``ReviewPromptState`` to `UserDefaults` (GH #628).
///
/// Every field is device-local; nothing here is sent to a server or used for
/// analytics. Dates are stored as `Double` Unix timestamps and the rolling
/// prompt-timestamp list as a `[Double]` array. Mirrors
/// ``UserDefaultsOnboardingRepository``.
public final class UserDefaultsReviewPromptStore: ReviewPromptStore, @unchecked Sendable {
  private enum Key {
    static let firstLaunchDate = "reviewPrompt.firstLaunchDate"
    static let engagementScore = "reviewPrompt.engagementScore"
    static let saveCount = "reviewPrompt.saveCount"
    static let lastActiveDayKey = "reviewPrompt.lastActiveDayKey"
    static let distinctActiveDays = "reviewPrompt.distinctActiveDays"
    static let lastPromptDate = "reviewPrompt.lastPromptDate"
    static let promptTimestamps = "reviewPrompt.promptTimestamps"
    static let hasRecordedUpgrade = "reviewPrompt.hasRecordedUpgrade"
  }

  private let defaults: UserDefaults

  public init(defaults: UserDefaults = .standard) {
    self.defaults = defaults
  }

  public func load() -> ReviewPromptState {
    ReviewPromptState(
      firstLaunchDate: date(forKey: Key.firstLaunchDate),
      engagementScore: defaults.integer(forKey: Key.engagementScore),
      saveCount: defaults.integer(forKey: Key.saveCount),
      lastActiveDayKey: defaults.string(forKey: Key.lastActiveDayKey),
      distinctActiveDays: defaults.integer(forKey: Key.distinctActiveDays),
      lastPromptDate: date(forKey: Key.lastPromptDate),
      promptTimestamps: timestamps(forKey: Key.promptTimestamps),
      hasRecordedUpgrade: defaults.bool(forKey: Key.hasRecordedUpgrade)
    )
  }

  public func save(_ state: ReviewPromptState) {
    setDate(state.firstLaunchDate, forKey: Key.firstLaunchDate)
    defaults.set(state.engagementScore, forKey: Key.engagementScore)
    defaults.set(state.saveCount, forKey: Key.saveCount)
    setOptionalString(state.lastActiveDayKey, forKey: Key.lastActiveDayKey)
    defaults.set(state.distinctActiveDays, forKey: Key.distinctActiveDays)
    setDate(state.lastPromptDate, forKey: Key.lastPromptDate)
    defaults.set(state.promptTimestamps.map(\.timeIntervalSince1970), forKey: Key.promptTimestamps)
    defaults.set(state.hasRecordedUpgrade, forKey: Key.hasRecordedUpgrade)
  }

  // MARK: - Helpers

  private func date(forKey key: String) -> Date? {
    guard let seconds = defaults.object(forKey: key) as? Double else { return nil }
    return Date(timeIntervalSince1970: seconds)
  }

  private func setDate(_ date: Date?, forKey key: String) {
    if let date {
      defaults.set(date.timeIntervalSince1970, forKey: key)
    } else {
      defaults.removeObject(forKey: key)
    }
  }

  private func setOptionalString(_ value: String?, forKey key: String) {
    if let value {
      defaults.set(value, forKey: key)
    } else {
      defaults.removeObject(forKey: key)
    }
  }

  private func timestamps(forKey key: String) -> [Date] {
    let seconds = defaults.array(forKey: key) as? [Double] ?? []
    return seconds.map { Date(timeIntervalSince1970: $0) }
  }
}
