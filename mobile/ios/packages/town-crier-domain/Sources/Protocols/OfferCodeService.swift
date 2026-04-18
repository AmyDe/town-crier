import Foundation

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

/// Errors surfaced by `OfferCodeService` when redeeming a code.
///
/// The HTTP layer maps the backend's structured `error` code to a typed case;
/// unknown codes (and unparseable bodies) fall through to `.network` with context
/// so the UI can still show something rather than hang.
public enum OfferCodeError: Error, Sendable, Equatable {
  case invalidFormat
  case notFound
  case alreadyRedeemed
  case alreadySubscribed
  case network(String)

  /// Maps the `error` string returned by `POST /v1/offer-codes/redeem` to a
  /// typed case. Unrecognised codes become `.network` so the caller always
  /// receives an actionable error.
  public init(serverErrorCode: String) {
    switch serverErrorCode {
    case "invalid_code_format":
      self = .invalidFormat
    case "invalid_code":
      self = .notFound
    case "code_already_redeemed":
      self = .alreadyRedeemed
    case "already_subscribed":
      self = .alreadySubscribed
    default:
      self = .network("Unexpected error: \(serverErrorCode)")
    }
  }
}
