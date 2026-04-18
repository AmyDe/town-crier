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

  // MARK: - Input normalization

  @Test("redeem forwards the display-formatted code to the service as canonical")
  func redeem_normalisesDashesAndUppercase() async {
    let (sut, spy) = makeSUT()
    sut.code = "a7km-zqr3-fnxp"

    await sut.redeem()

    #expect(spy.redeemCalls == ["A7KMZQR3FNXP"])
  }

  @Test("redeem trims surrounding whitespace before calling the service")
  func redeem_trimsWhitespace() async {
    let (sut, spy) = makeSUT()
    sut.code = "  A7KM ZQR3 FNXP  "

    await sut.redeem()

    #expect(spy.redeemCalls == ["A7KMZQR3FNXP"])
  }

  // MARK: - Client-side validation

  @Test("redeem with empty code shows format error and skips the service")
  func redeem_empty_showsFormatErrorAndDoesNotCallService() async {
    let (sut, spy) = makeSUT()
    sut.code = ""

    await sut.redeem()

    #expect(spy.redeemCalls.isEmpty)
    #expect(sut.errorMessage == "Please check the code and try again.")
  }

  @Test("redeem with wrong length shows format error and skips the service")
  func redeem_wrongLength_showsFormatErrorAndDoesNotCallService() async {
    let (sut, spy) = makeSUT()
    sut.code = "A7KM-ZQR3"

    await sut.redeem()

    #expect(spy.redeemCalls.isEmpty)
    #expect(sut.errorMessage == "Please check the code and try again.")
  }

  @Test("redeem with characters outside Crockford base32 shows format error")
  func redeem_disallowedCharacters_showsFormatError() async {
    let (sut, spy) = makeSUT()
    // Crockford base32 excludes I, L, O, U — `ILOU` is explicitly invalid.
    sut.code = "ILOU-ZQR3-FNXP"

    await sut.redeem()

    #expect(spy.redeemCalls.isEmpty)
    #expect(sut.errorMessage == "Please check the code and try again.")
  }

  // MARK: - Happy path

  @Test("redeem exposes the redemption result on success")
  func redeem_success_publishesRedemption() async {
    let expected = OfferCodeRedemption(
      tier: .pro,
      expiresAt: Date(timeIntervalSince1970: 1_800_000_000)
    )
    let (sut, _) = makeSUT(redeemResult: .success(expected))
    sut.code = "A7KM-ZQR3-FNXP"

    await sut.redeem()

    #expect(sut.redemption == expected)
    #expect(sut.errorMessage == nil)
  }

  @Test("redeem invokes onRedeemed with the result on success")
  func redeem_success_invokesOnRedeemed() async {
    let expected = OfferCodeRedemption(
      tier: .pro,
      expiresAt: Date(timeIntervalSince1970: 1_800_000_000)
    )
    let (sut, _) = makeSUT(redeemResult: .success(expected))
    sut.code = "A7KM-ZQR3-FNXP"

    var received: OfferCodeRedemption?
    sut.onRedeemed = { received = $0 }

    await sut.redeem()

    #expect(received == expected)
  }

  @Test("redeem clears any previous errorMessage on retry")
  func redeem_clearsPreviousErrorOnSuccess() async {
    let (sut, spy) = makeSUT()
    sut.code = "bad"
    await sut.redeem()
    #expect(sut.errorMessage != nil)

    // Switch to a valid code and a successful service result.
    spy.redeemResult = .success(
      OfferCodeRedemption(
        tier: .personal,
        expiresAt: Date(timeIntervalSince1970: 1_800_000_000)
      )
    )
    sut.code = "A7KM-ZQR3-FNXP"
    await sut.redeem()

    #expect(sut.errorMessage == nil)
  }

  @Test("redeem resets isLoading to false after the call returns")
  func redeem_resetsLoadingAfterCall() async {
    let (sut, _) = makeSUT()
    sut.code = "A7KM-ZQR3-FNXP"

    await sut.redeem()

    #expect(sut.isLoading == false)
  }
}
