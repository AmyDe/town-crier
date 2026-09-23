import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("StoreKitSubscriptionService — server verification")
struct StoreKitSubscriptionServiceVerificationTests {

  @Test("reportPurchase POSTs the signed transaction to the verification service")
  func reportPurchase_postsSignedTransaction() async throws {
    let verifier = SpySubscriptionVerifier()
    let sut = StoreKitSubscriptionService(verificationService: verifier)

    try await sut.reportPurchase(signedTransaction: "header.payload.signature")

    #expect(verifier.verifiedTransactions == ["header.payload.signature"])
  }

  @Test("reportPurchase swallows a transient verification failure so the purchase still succeeds")
  func reportPurchase_swallowsVerificationFailure() async throws {
    let verifier = SpySubscriptionVerifier()
    verifier.setVerifyResult(.failure(DomainError.networkUnavailable))
    let sut = StoreKitSubscriptionService(verificationService: verifier)

    try await sut.reportPurchase(signedTransaction: "header.payload.signature")

    #expect(verifier.verifiedTransactions == ["header.payload.signature"])
  }

  @Test("reportPurchase is a no-op when no verification service is injected")
  func reportPurchase_noVerifier_isNoOp() async throws {
    let sut = StoreKitSubscriptionService()

    try await sut.reportPurchase(signedTransaction: "header.payload.signature")
  }

  @Test("reportPurchase rethrows transactionAlreadyClaimed (tc-k42ce.2, GH#1165)")
  func reportPurchase_rethrowsTransactionAlreadyClaimed() async {
    let verifier = SpySubscriptionVerifier()
    verifier.setVerifyResult(.failure(DomainError.transactionAlreadyClaimed))
    let sut = StoreKitSubscriptionService(verificationService: verifier)

    await #expect(throws: DomainError.transactionAlreadyClaimed) {
      try await sut.reportPurchase(signedTransaction: "header.payload.signature")
    }
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

  @Test("reportRestoreBestEffort POSTs the collected JWS list to the verification service")
  func reportRestoreBestEffort_postsSignedTransactionList() async {
    let verifier = SpySubscriptionVerifier()
    let sut = StoreKitSubscriptionService(verificationService: verifier)

    await sut.reportRestoreBestEffort(signedTransactions: ["jws.one", "jws.two"])

    #expect(verifier.restoredTransactionBatches == [["jws.one", "jws.two"]])
  }

  @Test("reportRestoreBestEffort swallows a verification failure so the entitlement survives")
  func reportRestoreBestEffort_swallowsVerificationFailure() async {
    let verifier = SpySubscriptionVerifier()
    verifier.setVerifyResult(.failure(DomainError.networkUnavailable))
    let sut = StoreKitSubscriptionService(verificationService: verifier)

    await sut.reportRestoreBestEffort(signedTransactions: ["jws.one"])

    #expect(verifier.restoredTransactionBatches == [["jws.one"]])
  }

  @Test("reportRestoreBestEffort is a no-op when no verification service is injected")
  func reportRestoreBestEffort_noVerifier_isNoOp() async {
    let sut = StoreKitSubscriptionService()

    await sut.reportRestoreBestEffort(signedTransactions: ["jws.one"])
  }
}
