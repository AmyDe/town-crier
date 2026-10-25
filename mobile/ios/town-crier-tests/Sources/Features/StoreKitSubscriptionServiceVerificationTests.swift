import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("StoreKitSubscriptionService — server verification")
struct StoreKitSubscriptionServiceVerificationTests {

  @Test("reportThenFinish reports the transaction before finishing it")
  func reportThenFinish_reportsBeforeFinishing() async throws {
    let verifier = SpySubscriptionVerifier()
    let finish = FinishRecorder(verifier: verifier)

    try await StoreKitSubscriptionService.reportThenFinish(
      signedTransaction: "header.payload.signature",
      verificationService: verifier,
      finish: finish.finish)

    #expect(verifier.verifiedTransactions == ["header.payload.signature"])
    #expect(finish.finishCount == 1)
    #expect(finish.reportedBeforeFinish)
  }

  @Test("reportThenFinish leaves the transaction unfinished when the report fails")
  func reportThenFinish_transientFailure_doesNotFinish() async throws {
    let verifier = SpySubscriptionVerifier()
    verifier.setVerifyResult(.failure(DomainError.networkUnavailable))
    let finish = FinishRecorder(verifier: verifier)

    try await StoreKitSubscriptionService.reportThenFinish(
      signedTransaction: "header.payload.signature",
      verificationService: verifier,
      finish: finish.finish)

    #expect(verifier.verifiedTransactions == ["header.payload.signature"])
    #expect(finish.finishCount == 0)
  }

  @Test("reportThenFinish finishes a transaction the server rejects as a bad request")
  func reportThenFinish_badRequest_finishes() async throws {
    let verifier = SpySubscriptionVerifier()
    verifier.setVerifyResult(
      .failure(APIError.serverError(statusCode: 400, message: "invalid_transaction_payload")))
    let finish = FinishRecorder(verifier: verifier)

    try await StoreKitSubscriptionService.reportThenFinish(
      signedTransaction: "header.payload.signature",
      verificationService: verifier,
      finish: finish.finish)

    #expect(finish.finishCount == 1)
  }

  @Test("reportThenFinish leaves the transaction unfinished on a 5xx or a 401")
  func reportThenFinish_serverErrorOrUnauthorized_doesNotFinish() async throws {
    for error: any Error in [
      APIError.serverError(statusCode: 503, message: nil), APIError.unauthorized,
    ] {
      let verifier = SpySubscriptionVerifier()
      verifier.setVerifyResult(.failure(error))
      let finish = FinishRecorder(verifier: verifier)

      try await StoreKitSubscriptionService.reportThenFinish(
        signedTransaction: "header.payload.signature",
        verificationService: verifier,
        finish: finish.finish)

      #expect(finish.finishCount == 0)
    }
  }

  @Test("reportThenFinish finishes immediately when no verification service is injected")
  func reportThenFinish_noVerifier_finishes() async throws {
    let finish = FinishRecorder(verifier: SpySubscriptionVerifier())

    try await StoreKitSubscriptionService.reportThenFinish(
      signedTransaction: "header.payload.signature",
      verificationService: nil,
      finish: finish.finish)

    #expect(finish.finishCount == 1)
  }

  @Test("reportThenFinish finishes then rethrows transactionAlreadyClaimed")
  func reportThenFinish_alreadyClaimed_finishesAndRethrows() async {
    let verifier = SpySubscriptionVerifier()
    verifier.setVerifyResult(.failure(DomainError.transactionAlreadyClaimed))
    let finish = FinishRecorder(verifier: verifier)

    await #expect(throws: DomainError.transactionAlreadyClaimed) {
      try await StoreKitSubscriptionService.reportThenFinish(
        signedTransaction: "header.payload.signature",
        verificationService: verifier,
        finish: finish.finish)
    }

    #expect(finish.finishCount == 1)
    #expect(finish.reportedBeforeFinish)
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

    _ = await sut.reportRestoreBestEffort(signedTransactions: ["jws.one"])
  }

  @Test("reportRestoreBestEffort returns true when claimed by another account (tc-k42ce.2, GH#1165)")
  func reportRestoreBestEffort_returnsTrue_whenClaimedByAnotherAccount() async {
    let verifier = SpySubscriptionVerifier()
    verifier.setVerifyResult(.failure(DomainError.transactionAlreadyClaimed))
    let sut = StoreKitSubscriptionService(verificationService: verifier)

    let claimedByAnotherAccount = await sut.reportRestoreBestEffort(
      signedTransactions: ["jws.one"])

    #expect(claimedByAnotherAccount)
  }

  @Test(
    "reportRestoreBestEffort returns false on a transient failure (tc-k42ce.2, GH#1165 / #1161 regression)"
  )
  func reportRestoreBestEffort_returnsFalse_onTransientFailure() async {
    let verifier = SpySubscriptionVerifier()
    verifier.setVerifyResult(.failure(DomainError.networkUnavailable))
    let sut = StoreKitSubscriptionService(verificationService: verifier)

    let claimedByAnotherAccount = await sut.reportRestoreBestEffort(
      signedTransactions: ["jws.one"])

    #expect(!claimedByAnotherAccount)
  }
}

private final class FinishRecorder: @unchecked Sendable {
  private let lock = NSLock()
  private let verifier: SpySubscriptionVerifier
  private var count = 0
  private var reportedFirst = true

  init(verifier: SpySubscriptionVerifier) {
    self.verifier = verifier
  }

  var finishCount: Int { lock.withLock { count } }
  var reportedBeforeFinish: Bool { lock.withLock { reportedFirst } }

  @Sendable func finish() async {
    let reported = !verifier.verifiedTransactions.isEmpty
    lock.withLock {
      count += 1
      if !reported { reportedFirst = false }
    }
  }
}
