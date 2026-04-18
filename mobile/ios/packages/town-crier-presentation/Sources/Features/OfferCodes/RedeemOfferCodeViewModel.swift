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
}
