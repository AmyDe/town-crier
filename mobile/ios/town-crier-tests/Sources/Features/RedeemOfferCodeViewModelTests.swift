import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("RedeemOfferCodeViewModel")
@MainActor
struct RedeemOfferCodeViewModelTests {
  private func makeSUT(
    redeemResult: Result<OfferCodeRedemption, Error> = .success(
      OfferCodeRedemption(
        tier: .personal,
        expiresAt: Date(timeIntervalSince1970: 1_800_000_000)
      )
    )
  ) -> (RedeemOfferCodeViewModel, SpyOfferCodeService) {
    let spy = SpyOfferCodeService()
    spy.redeemResult = redeemResult
    let sut = RedeemOfferCodeViewModel(offerCodeService: spy)
    return (sut, spy)
  }

  // MARK: - Initial state

  @Test("initial state is empty and idle")
  func initialState_isEmptyAndIdle() {
    let (sut, _) = makeSUT()

    #expect(sut.code.isEmpty)
    #expect(sut.isLoading == false)
    #expect(sut.errorMessage == nil)
    #expect(sut.redemption == nil)
  }
}
