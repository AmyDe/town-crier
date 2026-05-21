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

  // MARK: - Restore

  @Test("reportRestore POSTs the collected JWS list to the verification service")
  func reportRestore_postsSignedTransactionList() async throws {
    let verifier = SpySubscriptionVerifier()
    let sut = StoreKitSubscriptionService(verificationService: verifier)

    try await sut.reportRestore(signedTransactions: ["jws.one", "jws.two"])

    #expect(verifier.restoredTransactionBatches == [["jws.one", "jws.two"]])
  }

  @Test("reportRestore does not POST when there are no active entitlements")
  func reportRestore_emptyList_doesNotPost() async throws {
    let verifier = SpySubscriptionVerifier()
    let sut = StoreKitSubscriptionService(verificationService: verifier)

    try await sut.reportRestore(signedTransactions: [])

    #expect(verifier.restoredTransactionBatches.isEmpty)
  }

  @Test("reportRestore surfaces a verification failure to the caller")
  func reportRestore_surfacesVerificationFailure() async {
    let verifier = SpySubscriptionVerifier()
    verifier.setVerifyResult(.failure(DomainError.purchaseFailed("invalid_transaction")))
    let sut = StoreKitSubscriptionService(verificationService: verifier)

    // Restore is an explicit user action — a tampered-JWS 401 must surface.
    await #expect(throws: (any Error).self) {
      try await sut.reportRestore(signedTransactions: ["tampered.jws"])
    }
  }

  @Test("reportRestore is a no-op when no verification service is injected")
  func reportRestore_noVerifier_isNoOp() async throws {
    let sut = StoreKitSubscriptionService()

    // Must not crash and must not throw.
    try await sut.reportRestore(signedTransactions: ["jws.one"])
  }
}
