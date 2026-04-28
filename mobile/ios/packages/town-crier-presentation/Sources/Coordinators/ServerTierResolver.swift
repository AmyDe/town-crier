import Foundation
import TownCrierDomain
import os

/// Ensures the user has a server-side profile (POST /v1/me) and returns its
/// subscription tier.
///
/// `POST /v1/me` is idempotent server-side: the
/// `CreateUserProfileCommandHandler` returns the existing profile when one
/// is already present, and creates one otherwise. Calling it on every tier
/// resolution backfills profiles for users who signed in before this code
/// path existed (see bug tc-a6it — iOS-only signups previously had no
/// Cosmos `UserProfile` document and were invisible to backend tooling).
///
/// Shared by both ``AppCoordinator`` and ``SettingsViewModel`` so the two
/// tier-resolution paths cannot drift apart again (tc-aza5).
public protocol ServerTierResolving: Sendable {
  /// Ensures the server profile exists and returns its tier.
  ///
  /// Returns `nil` when the call fails due to a network or server error,
  /// so callers can distinguish "ensure failed" from "user is genuinely
  /// on free tier" and apply the appropriate fallback (preserve cached
  /// tier vs. fall back to JWT/StoreKit).
  func ensureServerProfileTier() async -> SubscriptionTier?
}

public final class ServerTierResolver: ServerTierResolving {
  private static let logger = Logger(
    subsystem: "uk.towncrierapp",
    category: "ServerTierResolver"
  )

  private let userProfileRepository: UserProfileRepository

  public init(userProfileRepository: UserProfileRepository) {
    self.userProfileRepository = userProfileRepository
  }

  public func ensureServerProfileTier() async -> SubscriptionTier? {
    do {
      let profile = try await userProfileRepository.create()
      return profile.tier
    } catch {
      Self.logger.error(
        "Failed to ensure server profile for subscription tier: \(error.localizedDescription)"
      )
      return nil
    }
  }
}
