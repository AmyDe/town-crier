import Foundation
import TownCrierDomain
import os

/// Folds the three subscription-tier sources (server profile, StoreKit
/// entitlement, JWT claim) into a single resolved tier and trial flag.
///
/// Shared by both ``AppCoordinator`` and ``SettingsViewModel`` so the two
/// tier-resolution paths cannot drift apart again (third recurrence after
/// tc-aza5; original bug tc-exg6).
///
/// ### Resolution rules
///
/// 1. Compute `effectiveServerTier = serverTier ?? max(previousTier, jwtTier)`.
///    When the server fetch fails, fall back to the richer of the cached
///    previous tier and the current JWT claim — never silently downgrade a
///    paying user to `.free` due to a transient network error.
/// 2. Take the maximum of `effectiveServerTier`, the StoreKit tier, and the
///    JWT tier.
/// 3. If the winner is `.free` on the first pass, log a diagnostic notice
///    and call `authService.refreshSession()` exactly once; if the new JWT
///    promotes the user, the second pass returns the higher tier.
/// 4. The trial flag is meaningful only when StoreKit's tier is the winner
///    (the server profile carries no trial information).
public protocol SubscriptionTierResolving: Sendable {
  func resolve(
    jwtTier: SubscriptionTier,
    previousTier: SubscriptionTier,
    userSub: String?
  ) async -> (tier: SubscriptionTier, isTrialPeriod: Bool)
}

public final class SubscriptionTierResolver: SubscriptionTierResolving {
  private static let logger = Logger(
    subsystem: "uk.towncrierapp",
    category: "SubscriptionTierResolver"
  )

  private let serverFetcher: @Sendable () async -> SubscriptionTier?
  private let storeKitFetcher: @Sendable () async -> SubscriptionEntitlement?
  private let authService: AuthenticationService

  public init(
    serverFetcher: @escaping @Sendable () async -> SubscriptionTier?,
    storeKitFetcher: @escaping @Sendable () async -> SubscriptionEntitlement?,
    authService: AuthenticationService
  ) {
    self.serverFetcher = serverFetcher
    self.storeKitFetcher = storeKitFetcher
    self.authService = authService
  }

  public func resolve(
    jwtTier: SubscriptionTier,
    previousTier: SubscriptionTier,
    userSub: String?
  ) async -> (tier: SubscriptionTier, isTrialPeriod: Bool) {
    let firstPass = await resolveOnce(jwtTier: jwtTier, previousTier: previousTier)
    guard firstPass.tier == .free else {
      return firstPass
    }

    // Winner is .free — log a diagnostic notice and try once to recover by
    // refreshing the session (which may rotate the JWT claim).
    Self.logger.notice(
      """
      SubscriptionTierResolver winner is .free \
      (jwt=\(jwtTier.rawValue, privacy: .public), \
      previous=\(previousTier.rawValue, privacy: .public), \
      sub=\(userSub ?? "nil", privacy: .private))
      """
    )

    let refreshedJwtTier: SubscriptionTier
    do {
      let refreshed = try await authService.refreshSession()
      refreshedJwtTier = refreshed.subscriptionTier
    } catch {
      Self.logger.error(
        "refreshSession() failed during free-tier retry: \(error.localizedDescription)"
      )
      return firstPass
    }

    // Second pass — guarded so we never recurse: the second pass returns
    // whatever it resolves, even if still .free.
    return await resolveOnce(jwtTier: refreshedJwtTier, previousTier: previousTier)
  }

  private func resolveOnce(
    jwtTier: SubscriptionTier,
    previousTier: SubscriptionTier
  ) async -> (tier: SubscriptionTier, isTrialPeriod: Bool) {
    let serverTier = await serverFetcher()
    let storeKitEntitlement = await storeKitFetcher()
    let storeKitTier = storeKitEntitlement?.tier ?? .free

    // When the server fetch fails (nil), fall back to max(previousTier,
    // jwtTier) — never .free. Mirrors AppCoordinator's previous behaviour
    // and prevents transient errors from silently downgrading users.
    let effectiveServerTier = serverTier ?? max(previousTier, jwtTier)
    let resolved = max(effectiveServerTier, max(storeKitTier, jwtTier))

    // Trial period is only meaningful when StoreKit's tier is the winner.
    let isTrialPeriod =
      storeKitEntitlement?.isTrialPeriod == true
      && storeKitTier >= max(effectiveServerTier, jwtTier)

    return (resolved, isTrialPeriod)
  }
}
