import Foundation
import TownCrierDomain

/// Redeems single-use offer codes against the Town Crier backend.
///
/// Implementations are responsible for normalising the input code (the backend
/// accepts codes with or without `-` separators) and mapping HTTP failure
/// responses to `OfferCodeError`. Callers receive either a successful
/// `OfferCodeRedemption` or a typed error they can switch on.
public protocol OfferCodeService: Sendable {
  func redeem(code: String) async throws -> OfferCodeRedemption
}

/// Successful outcome of redeeming an offer code.
///
/// Mirrors the `{"tier": "...", "expiresAt": "..."}` response body from
/// `POST /v1/offer-codes/redeem`.
public struct OfferCodeRedemption: Sendable, Equatable {
  public let tier: SubscriptionTier
  public let expiresAt: Date

  public init(tier: SubscriptionTier, expiresAt: Date) {
    self.tier = tier
    self.expiresAt = expiresAt
  }
}
