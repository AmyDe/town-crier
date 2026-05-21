import Foundation
import TownCrierDomain

/// Records the signed transactions passed to `verify` and returns a
/// preconfigured result (or throws a preconfigured error).
final class SpySubscriptionVerifier: SubscriptionVerificationService, @unchecked Sendable {
  private let lock = NSLock()
  private var recorded: [String] = []
  private var result: Result<VerifiedSubscription, Error> = .success(
    VerifiedSubscription(
      tier: .personal,
      subscriptionExpiry: nil,
      entitlements: [],
      watchZoneLimit: 3
    )
  )

  private var recordedRestoreBatches: [[String]] = []

  var verifiedTransactions: [String] {
    lock.withLock { recorded }
  }

  /// The `signedTransactions` lists passed to `verifyRestore`, one entry per call.
  var restoredTransactionBatches: [[String]] {
    lock.withLock { recordedRestoreBatches }
  }

  func setVerifyResult(_ value: Result<VerifiedSubscription, Error>) {
    lock.withLock { result = value }
  }

  func verify(signedTransaction: String) async throws -> VerifiedSubscription {
    let current: Result<VerifiedSubscription, Error> = lock.withLock {
      recorded.append(signedTransaction)
      return result
    }
    return try current.get()
  }

  func verifyRestore(signedTransactions: [String]) async throws -> VerifiedSubscription {
    let current: Result<VerifiedSubscription, Error> = lock.withLock {
      recordedRestoreBatches.append(signedTransactions)
      return result
    }
    return try current.get()
  }
}
