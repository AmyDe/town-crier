import Foundation
import TownCrierDomain

@testable import TownCrierPresentation

/// Test double for `SubscriptionTierResolving`. Records each `resolve(...)`
/// call's arguments and returns a configurable result so tests can assert on
/// how `AppCoordinator` and `SettingsViewModel` delegate to the shared
/// resolver without re-testing the resolver's internal logic.
final class FakeSubscriptionTierResolver: SubscriptionTierResolving, @unchecked Sendable {
  struct ResolveCall: Equatable {
    let jwtTier: SubscriptionTier
    let previousTier: SubscriptionTier
    let userSub: String?
  }

  private(set) var resolveCalls: [ResolveCall] = []
  var resolveResult: (tier: SubscriptionTier, isTrialPeriod: Bool) = (.free, false)

  func resolve(
    jwtTier: SubscriptionTier,
    previousTier: SubscriptionTier,
    userSub: String?
  ) async -> (tier: SubscriptionTier, isTrialPeriod: Bool) {
    resolveCalls.append(
      ResolveCall(
        jwtTier: jwtTier,
        previousTier: previousTier,
        userSub: userSub
      )
    )
    return resolveResult
  }
}
