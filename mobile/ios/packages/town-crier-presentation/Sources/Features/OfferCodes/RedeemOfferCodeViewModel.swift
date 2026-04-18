import Combine
import Foundation
import TownCrierDomain

/// ViewModel for the "Redeem offer code" screen reached from Settings.
///
/// Owns the user-facing state for a single redemption attempt:
///
/// - `code` — raw input bound to the text field; accepts any case and `-`
///   separators (backend and the ViewModel both normalise).
/// - `isLoading` — true while a redemption is in flight.
/// - `errorMessage` — a user-friendly message derived from the last failure;
///   `nil` when there is no active error.
/// - `redemption` — the successful outcome; drives the success alert.
///
/// The `onRedeemed` callback lets the Coordinator trigger downstream work
/// (force a token refresh, reload the server profile, dismiss the screen)
/// without the ViewModel reaching into navigation concerns.
@MainActor
public final class RedeemOfferCodeViewModel: ObservableObject {
  @Published public var code: String = ""
  @Published public private(set) var isLoading = false
  @Published public private(set) var errorMessage: String?
  @Published public private(set) var redemption: OfferCodeRedemption?

  public var onRedeemed: ((OfferCodeRedemption) -> Void)?

  private let offerCodeService: OfferCodeService

  public init(offerCodeService: OfferCodeService) {
    self.offerCodeService = offerCodeService
  }

  /// Validates and submits `code` to the backend.
  ///
  /// Normalisation mirrors the server's rule set: strip whitespace and `-`,
  /// uppercase, then check the result is 12 chars drawn from the Crockford
  /// base32 alphabet (`0-9A-Z` excluding `I L O U`). Ill-formed input is
  /// rejected locally so we don't waste a network round-trip.
  public func redeem() async {
    let canonical = Self.normalise(code)
    guard Self.isValidCanonical(canonical) else {
      errorMessage = Self.invalidFormatMessage
      return
    }
    errorMessage = nil
    isLoading = true
    defer { isLoading = false }
    _ = try? await offerCodeService.redeem(code: canonical)
  }

  // MARK: - Normalisation / validation

  private static func normalise(_ raw: String) -> String {
    raw
      .uppercased()
      .unicodeScalars
      .filter { !CharacterSet.whitespacesAndNewlines.contains($0) && $0 != "-" }
      .reduce(into: "") { accumulator, scalar in
        accumulator.unicodeScalars.append(scalar)
      }
  }

  private static let crockfordAlphabet: Set<Character> = Set("0123456789ABCDEFGHJKMNPQRSTVWXYZ")

  private static func isValidCanonical(_ value: String) -> Bool {
    value.count == 12 && value.allSatisfy(crockfordAlphabet.contains)
  }

  // MARK: - Error messages

  private static let invalidFormatMessage = "Please check the code and try again."
}
