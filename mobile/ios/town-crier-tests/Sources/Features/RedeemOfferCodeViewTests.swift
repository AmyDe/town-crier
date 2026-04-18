import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("RedeemOfferCodeView")
@MainActor
struct RedeemOfferCodeViewTests {

  // MARK: - Helpers

  private func makeViewModel(
    redeemResult: Result<OfferCodeRedemption, Error> = .success(
      OfferCodeRedemption(
        tier: .personal,
        expiresAt: Date(timeIntervalSince1970: 1_800_000_000)
      )
    )
  ) -> RedeemOfferCodeViewModel {
    let spy = SpyOfferCodeService()
    spy.redeemResult = redeemResult
    return RedeemOfferCodeViewModel(offerCodeService: spy)
  }

  // MARK: - View Construction

  @Test("RedeemOfferCodeView can be constructed in its idle state")
  func construction_idleState_succeeds() {
    let viewModel = makeViewModel()

    let view = RedeemOfferCodeView(viewModel: viewModel)

    _ = view
  }

  @Test("RedeemOfferCodeView can be constructed after a successful redemption")
  func construction_afterSuccess_succeeds() async {
    let viewModel = makeViewModel()
    viewModel.code = "A7KM-ZQR3-FNXP"
    await viewModel.redeem()

    let view = RedeemOfferCodeView(viewModel: viewModel)

    _ = view
  }

  @Test("RedeemOfferCodeView can be constructed after a redemption error")
  func construction_afterError_succeeds() async {
    let viewModel = makeViewModel(redeemResult: .failure(OfferCodeError.notFound))
    viewModel.code = "A7KM-ZQR3-FNXP"
    await viewModel.redeem()

    let view = RedeemOfferCodeView(viewModel: viewModel)

    _ = view
  }
}
