import Foundation
import TownCrierDomain

/// Hand-written fake for `OfferCodeService` used by `RedeemOfferCodeViewModelTests`.
///
/// Records each call's normalised code and returns a caller-supplied result.
/// Following the same pattern as `SpyAuthenticationService` — explicit over
/// reflection, and keeps tests readable.
final class SpyOfferCodeService: OfferCodeService, @unchecked Sendable {
  private(set) var redeemCalls: [String] = []
  var redeemResult: Result<OfferCodeRedemption, Error> = .success(
    OfferCodeRedemption(
      tier: .personal,
      expiresAt: Date(timeIntervalSince1970: 1_800_000_000)
    )
  )

  func redeem(code: String) async throws -> OfferCodeRedemption {
    redeemCalls.append(code)
    return try redeemResult.get()
  }
}
