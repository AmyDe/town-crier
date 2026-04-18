import Foundation

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
