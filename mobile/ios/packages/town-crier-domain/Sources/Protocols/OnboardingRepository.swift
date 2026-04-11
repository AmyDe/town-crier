/// Tracks whether the user has completed the first-launch onboarding flow.
public protocol OnboardingRepository: Sendable {
  var isOnboardingComplete: Bool { get }
  func markOnboardingComplete()
}
