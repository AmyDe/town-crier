import Foundation
import TownCrierDomain

extension AppCoordinator {
  /// Presents the "Redeem Offer Code" sheet from Settings. Has no effect if
  /// the Coordinator was constructed without an `OfferCodeService` (i.e. the
  /// feature is not wired).
  public func showRedeemOfferCode() {
    guard offerCodeService != nil else { return }
    isRedeemOfferCodePresented = true
  }

  /// Creates a `RedeemOfferCodeViewModel` wired to dismiss the sheet and
  /// refresh the subscription tier on successful redemption.
  ///
  /// Returns `nil` when no `OfferCodeService` was injected — callers should
  /// hide the Settings entry point in that case.
  public func makeRedeemOfferCodeViewModel() -> RedeemOfferCodeViewModel? {
    guard let offerCodeService else { return nil }
    let viewModel = RedeemOfferCodeViewModel(offerCodeService: offerCodeService)
    viewModel.onRedeemed = { [weak self] _ in
      self?.handleOfferCodeRedeemed()
    }
    return viewModel
  }

  /// Test-only synchronisation: await the post-redemption refresh so
  /// assertions happen after the session and tier have been re-resolved.
  public func waitForPendingOfferCodeRefresh() async {
    await pendingOfferCodeRefresh?.value
  }

  private func handleOfferCodeRedeemed() {
    isRedeemOfferCodePresented = false
    // Detached task so tests can await it. Session refresh rotates the JWT so
    // the next server call sees the new `subscription_tier` claim, then
    // re-resolving the tier picks up the updated profile.
    pendingOfferCodeRefresh = Task { [weak self] in
      guard let self else { return }
      do {
        _ = try await authService.refreshSession()
      } catch {
        Self.logger.error(
          "Offer-code session refresh failed: \(error.localizedDescription)"
        )
      }
      await resolveSubscriptionTier()
    }
  }
}
