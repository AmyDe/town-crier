import Foundation

/// Reports an Apple-signed StoreKit 2 transaction to the Town Crier backend so
/// the server can cryptographically verify it and update the user's
/// entitlement state in Cosmos DB (ADR 0010 — verify-JWS-locally trust model).
///
/// On-device StoreKit verification is the source of truth for *local* feature
/// gating; this port exists so the server is told about a purchase and can
/// enforce tier-gated API requests (e.g. the watch-zone quota).
public protocol SubscriptionVerificationService: Sendable {
  /// Sends a compact JWS StoreKit 2 transaction to `POST /v1/subscriptions/verify`
  /// and returns the server-resolved entitlement state.
  func verify(signedTransaction: String) async throws -> VerifiedSubscription
}

/// Server-resolved subscription state returned by `POST /v1/subscriptions/verify`.
///
/// Mirrors the API response body
/// `{"tier": "...", "subscriptionExpiry": "...", "entitlements": [...], "watchZoneLimit": N}`.
public struct VerifiedSubscription: Sendable, Equatable {
  public let tier: SubscriptionTier
  public let subscriptionExpiry: Date?
  public let entitlements: [String]
  public let watchZoneLimit: Int

  public init(
    tier: SubscriptionTier,
    subscriptionExpiry: Date?,
    entitlements: [String],
    watchZoneLimit: Int
  ) {
    self.tier = tier
    self.subscriptionExpiry = subscriptionExpiry
    self.entitlements = entitlements
    self.watchZoneLimit = watchZoneLimit
  }
}
