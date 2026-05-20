import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("StoreKitSubscriptionService — server verification")
struct StoreKitSubscriptionServiceVerificationTests {

  @Test("reportPurchase POSTs the signed transaction to the verification service")
  func reportPurchase_postsSignedTransaction() async {
    let verifier = SpySubscriptionVerifier()
    let sut = StoreKitSubscriptionService(verificationService: verifier)

    await sut.reportPurchase(signedTransaction: "header.payload.signature")

    #expect(verifier.verifiedTransactions == ["header.payload.signature"])
  }

  @Test("reportPurchase swallows verification failures so the purchase still succeeds")
  func reportPurchase_swallowsVerificationFailure() async {
    let verifier = SpySubscriptionVerifier()
    verifier.setVerifyResult(.failure(DomainError.networkUnavailable))
    let sut = StoreKitSubscriptionService(verificationService: verifier)

    // Must not throw — server reporting is best-effort.
    await sut.reportPurchase(signedTransaction: "header.payload.signature")

    #expect(verifier.verifiedTransactions == ["header.payload.signature"])
  }

  @Test("reportPurchase is a no-op when no verification service is injected")
  func reportPurchase_noVerifier_isNoOp() async {
    let sut = StoreKitSubscriptionService()

    // Simply must not crash.
    await sut.reportPurchase(signedTransaction: "header.payload.signature")
  }
}
