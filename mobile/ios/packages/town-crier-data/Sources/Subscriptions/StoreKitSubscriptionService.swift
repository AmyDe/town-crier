import Foundation
import StoreKit
import TownCrierDomain
import os

/// StoreKit 2 adapter implementing the SubscriptionService domain protocol.
public final class StoreKitSubscriptionService: SubscriptionService, @unchecked Sendable {
  static let productIds = [
    "uk.towncrierapp.personal.monthly",
    "uk.towncrierapp.pro.monthly",
    "uk.towncrierapp.pro.annual",
    "uk.towncrierapp.pro.lifetime",
  ]

  private static let tierMapping: [String: SubscriptionTier] = [
    "uk.towncrierapp.personal.monthly": .personal,
    "uk.towncrierapp.pro.monthly": .pro,
    "uk.towncrierapp.pro.annual": .pro,
    "uk.towncrierapp.pro.lifetime": .pro,
  ]

  private static let periodMapping: [String: TownCrierDomain.SubscriptionPeriod] = [
    "uk.towncrierapp.personal.monthly": .monthly,
    "uk.towncrierapp.pro.monthly": .monthly,
    "uk.towncrierapp.pro.annual": .annual,
    "uk.towncrierapp.pro.lifetime": .lifetime,
  ]

  private static let logger = Logger(
    subsystem: "uk.towncrierapp", category: "StoreKitSubscriptionService")

  private let transactionListenerTask: Task<Void, Never>

  /// Optional: when nil the service relies purely on on-device StoreKit verification.
  private let verificationService: SubscriptionVerificationService?

  public init(verificationService: SubscriptionVerificationService? = nil) {
    self.verificationService = verificationService
    transactionListenerTask = Task.detached {
      await Self.listenForTransactionUpdates(verificationService: verificationService)
    }
  }

  deinit {
    transactionListenerTask.cancel()
  }

  public func availableProducts() async throws -> [SubscriptionProduct] {
    do {
      let storeProducts = try await Product.products(for: Self.productIds)
      let products: [SubscriptionProduct] = storeProducts.compactMap { product in
        guard let tier = Self.tier(forProductId: product.id),
          let period = Self.period(forProductId: product.id)
        else { return nil }

        let subscription = product.subscription
        let hasFreeTrial = subscription?.introductoryOffer?.paymentMode == .freeTrial
        let trialDays: Int
        if hasFreeTrial, let period = subscription?.introductoryOffer?.period {
          trialDays = period.value * (period.unit == .day ? 1 : period.unit == .week ? 7 : 30)
        } else {
          trialDays = 0
        }

        return SubscriptionProduct(
          id: product.id,
          displayName: product.displayName,
          displayPrice: product.displayPrice,
          tier: tier,
          hasFreeTrial: hasFreeTrial,
          trialDays: trialDays,
          period: period,
          price: product.price,
          currencyCode: product.priceFormatStyle.currencyCode
        )
      }
      return Self.sorted(products)
    } catch {
      throw DomainError.unexpected(error.localizedDescription)
    }
  }

  public func purchase(_ productId: String) async throws -> SubscriptionEntitlement {
    let storeProducts = try await Product.products(for: [productId])
    guard let product = storeProducts.first else {
      throw DomainError.productNotFound(productId)
    }

    let result = try await product.purchase()

    switch result {
    case .success(let verification):
      let transaction = try Self.checkVerification(verification)
      try await Self.reportThenFinish(
        signedTransaction: verification.jwsRepresentation,
        verificationService: verificationService
      ) { await transaction.finish() }
      return Self.entitlement(from: transaction)

    case .userCancelled:
      throw DomainError.purchaseCancelled

    case .pending:
      throw DomainError.purchaseFailed("Purchase is pending approval")

    @unknown default:
      throw DomainError.purchaseFailed("Unknown purchase result")
    }
  }

  public func restorePurchases() async throws -> SubscriptionEntitlement? {
    let collected = await collectEntitlements()
    try await reportRestore(signedTransactions: collected.signedTransactions)
    return collected.latest
  }

  public func currentEntitlement() async -> SubscriptionEntitlement? {
    let collected = await collectEntitlements()
    let claimedByAnotherAccount = await reportRestoreBestEffort(
      signedTransactions: collected.signedTransactions)
    return claimedByAnotherAccount ? nil : collected.latest
  }

  private func collectEntitlements() async -> (
    latest: SubscriptionEntitlement?, signedTransactions: [String]
  ) {
    var latestEntitlement: SubscriptionEntitlement?
    var signedTransactions: [String] = []

    for await result in Transaction.currentEntitlements {
      // Unverified entitlements are sent too: the server is the authority on a restore.
      signedTransactions.append(result.jwsRepresentation)

      if let transaction = try? Self.checkVerification(result) {
        let ent = Self.entitlement(from: transaction)
        if ent.isActive {
          latestEntitlement = Self.preferredEntitlement(current: latestEntitlement, candidate: ent)
        }
      }
    }

    return (latestEntitlement, signedTransactions)
  }

  public func hasActiveAutoRenewingSubscription() async -> Bool {
    for await result in Transaction.currentEntitlements {
      guard case .verified(let transaction) = result,
        transaction.productType == .autoRenewable,
        let expirationDate = transaction.expirationDate,
        expirationDate > Date()
      else { continue }

      // Unreadable renewal info counts as renewing: asking twice beats a double charge.
      guard let status = await transaction.subscriptionStatus else { return true }
      switch status.renewalInfo {
      case .verified(let renewalInfo):
        if renewalInfo.willAutoRenew { return true }
      case .unverified:
        return true
      }
    }
    return false
  }

  // MARK: - Transaction listener

  private static func listenForTransactionUpdates(
    verificationService: SubscriptionVerificationService?
  ) async {
    for await result in Transaction.updates {
      guard let transaction = try? checkVerification(result) else { continue }
      do {
        try await reportThenFinish(
          signedTransaction: result.jwsRepresentation,
          verificationService: verificationService
        ) { await transaction.finish() }
      } catch {
        logger.error(
          "Updated transaction already claimed: \(error.localizedDescription, privacy: .public)")
      }
    }
  }

  // MARK: - Server reporting

  /// Reports the transaction to the backend, then calls `finish`. `finish` runs
  /// when the report succeeds, when there is no verification service, when the
  /// server rejects the payload with a 400, or when the answer is
  /// ``DomainError/transactionAlreadyClaimed`` (which is then rethrown).
  /// Any other failure is logged and swallowed, and `finish` does not run.
  static func reportThenFinish(
    signedTransaction: String,
    verificationService: SubscriptionVerificationService?,
    finish: @Sendable () async -> Void
  ) async throws {
    guard let verificationService else {
      await finish()
      return
    }
    do {
      _ = try await verificationService.verify(signedTransaction: signedTransaction)
    } catch DomainError.transactionAlreadyClaimed {
      await finish()
      throw DomainError.transactionAlreadyClaimed
    } catch APIError.serverError(statusCode: 400, _) {
      // A 401 is not finished here: after a token refresh it cannot be told apart from an expired session.
      logger.error("Subscription verify POST rejected the payload, finishing the transaction")
    } catch {
      // Unfinished transactions are redelivered by Transaction.updates, which retries the report.
      logger.error(
        "Subscription verify POST failed: \(error.localizedDescription, privacy: .public)")
      return
    }
    await finish()
  }

  /// POSTs the JWS list from `Transaction.currentEntitlements` to the backend
  /// via the injected ``SubscriptionVerificationService`` for re-verification.
  ///
  /// Unlike ``reportThenFinish(signedTransaction:verificationService:finish:)``, a verification rejection
  /// (e.g. an HTTP 401 for a tampered JWS) is rethrown so the UI can tell the
  /// user. No-op for an empty list or when no service is injected.
  func reportRestore(signedTransactions: [String]) async throws {
    guard let verificationService, !signedTransactions.isEmpty else { return }
    _ = try await verificationService.verifyRestore(signedTransactions: signedTransactions)
  }

  /// Reports the entitlement list for a passive read. Returns `true` only when the report failed
  /// with ``DomainError/transactionAlreadyClaimed``; every other failure is swallowed.
  @discardableResult
  func reportRestoreBestEffort(signedTransactions: [String]) async -> Bool {
    do {
      try await reportRestore(signedTransactions: signedTransactions)
      return false
    } catch DomainError.transactionAlreadyClaimed {
      return true
    } catch {
      Self.logger.error(
        "Passive entitlement report failed: \(error.localizedDescription, privacy: .public)")
      return false
    }
  }

  // MARK: - Helpers

  private static func checkVerification<T>(
    _ result: VerificationResult<T>
  ) throws -> T {
    switch result {
    case .verified(let value):
      return value
    case .unverified:
      throw DomainError.purchaseFailed("Transaction verification failed")
    }
  }

  static func tier(forProductId productId: String) -> SubscriptionTier? {
    tierMapping[productId]
  }

  static func period(forProductId productId: String) -> TownCrierDomain.SubscriptionPeriod? {
    periodMapping[productId]
  }

  static func sorted(_ products: [SubscriptionProduct]) -> [SubscriptionProduct] {
    products.sorted { lhs, rhs in
      if lhs.tier != rhs.tier { return tierOrder(lhs.tier) < tierOrder(rhs.tier) }
      return lhs.period < rhs.period
    }
  }

  static func entitlement(from transaction: Transaction) -> SubscriptionEntitlement {
    entitlement(
      productId: transaction.productID,
      expirationDate: transaction.expirationDate,
      isIntroductoryOffer: transaction.offerType == .introductory,
      isNonConsumable: transaction.productType == .nonConsumable
    )
  }

  static func entitlement(
    productId: String,
    expirationDate: Date?,
    isIntroductoryOffer: Bool,
    isNonConsumable: Bool
  ) -> SubscriptionEntitlement {
    SubscriptionEntitlement(
      tier: tierMapping[productId] ?? .free,
      expiryDate: expirationDate ?? Date.distantFuture,
      isTrialPeriod: isIntroductoryOffer,
      productId: productId,
      isLifetime: isNonConsumable
    )
  }

  static func preferredEntitlement(
    current: SubscriptionEntitlement?,
    candidate: SubscriptionEntitlement
  ) -> SubscriptionEntitlement {
    guard let current else { return candidate }
    if tierOrder(candidate.tier) != tierOrder(current.tier) {
      return tierOrder(candidate.tier) > tierOrder(current.tier) ? candidate : current
    }
    return candidate.isLifetime && !current.isLifetime ? candidate : current
  }

  private static func tierOrder(_ tier: SubscriptionTier) -> Int {
    switch tier {
    case .free:
      return 0
    case .personal:
      return 1
    case .pro:
      return 2
    }
  }
}
