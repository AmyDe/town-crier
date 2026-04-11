import Foundation
import TownCrierDomain

/// Persists onboarding completion state to UserDefaults.
/// Onboarding is a device-local concern — reinstalling the app resets it.
public final class UserDefaultsOnboardingRepository: OnboardingRepository, @unchecked Sendable {
  private let defaults: UserDefaults
  private let key = "isOnboardingComplete"

  public init(defaults: UserDefaults = .standard) {
    self.defaults = defaults
  }

  public var isOnboardingComplete: Bool {
    defaults.bool(forKey: key)
  }

  public func markOnboardingComplete() {
    defaults.set(true, forKey: key)
  }
}
