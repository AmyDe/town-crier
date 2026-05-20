import Foundation
import StoreKit
import TownCrierDomain
import os

/// StoreKit 2 adapter implementing the SubscriptionService domain protocol.
public final class StoreKitSubscriptionService: SubscriptionService, @unchecked Sendable {
  private static let productIds = [
    "uk.co.towncrier.personal.monthly",
    "uk.co.towncrier.pro.monthly",
  ]

  private static let tierMapping: [String: SubscriptionTier] = [
    "uk.co.towncrier.personal.monthly": .personal,
    "uk.co.towncrier.pro.monthly": .pro,
  ]

  private static let logger = Logger(
    subsystem: "uk.towncrierapp", category: "StoreKitSubscriptionService")

  private var transactionListenerTask: Task<Void, Never>?

  /// Reports verified StoreKit transactions to the Town Crier backend so the
  /// server can update the user's entitlement state (ADR 0010). Optional —
  /// when nil the service relies purely on on-device StoreKit verification.
  private let verificationService: SubscriptionVerificationService?

  public init(verificationService: SubscriptionVerificationService? = nil) {
    self.verificationService = verificationService
    transactionListenerTask = Task.detached { [weak self] in
      await self?.listenForTransactionUpdates()
    }
  }

  deinit {
    transactionListenerTask?.cancel()
  }

  public func availableProducts() async throws -> [SubscriptionProduct] {
    do {
      let storeProducts = try await Product.products(for: Self.productIds)
      return storeProducts.compactMap { product in
        guard let tier = Self.tierMapping[product.id] else { return nil }

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
          trialDays: trialDays
        )
      }
      .sorted { tierOrder($0.tier) < tierOrder($1.tier) }
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
      let transaction = try checkVerification(verification)
      await transaction.finish()
      // Tell the backend about the purchase so tier-gated API requests see
      // the new tier (ADR 0010). Best-effort — never fails the purchase.
      await reportPurchase(signedTransaction: verification.jwsRepresentation)
      return entitlement(from: transaction)

    case .userCancelled:
      throw DomainError.purchaseCancelled

    case .pending:
      throw DomainError.purchaseFailed("Purchase is pending approval")

    @unknown default:
      throw DomainError.purchaseFailed("Unknown purchase result")
    }
  }

  public func restorePurchases() async throws -> SubscriptionEntitlement? {
    var latestEntitlement: SubscriptionEntitlement?

    for await result in Transaction.currentEntitlements {
      if let transaction = try? checkVerification(result) {
        let ent = entitlement(from: transaction)
        if ent.isActive {
          if let current = latestEntitlement {
            if tierOrder(ent.tier) > tierOrder(current.tier) {
              latestEntitlement = ent
            }
          } else {
            latestEntitlement = ent
          }
        }
      }
    }

    return latestEntitlement
  }

  public func currentEntitlement() async -> SubscriptionEntitlement? {
    try? await restorePurchases()
  }

  // MARK: - Transaction listener

  private func listenForTransactionUpdates() async {
    for await result in Transaction.updates {
      if let transaction = try? checkVerification(result) {
        await transaction.finish()
      }
    }
  }

  // MARK: - Server reporting

  /// POSTs an Apple-signed StoreKit 2 JWS transaction to the Town Crier
  /// backend via the injected ``SubscriptionVerificationService``.
  ///
  /// Best-effort by design: on-device StoreKit verification has already
  /// succeeded and is the source of truth for local feature gating, while
  /// Cosmos remains the source of truth for tier-gated API requests (ADR
  /// 0010). A network failure here is swallowed — the App Store Server
  /// Notifications webhook and the next server tier resolution reconcile it.
  func reportPurchase(signedTransaction: String) async {
    guard let verificationService else { return }
    do {
      _ = try await verificationService.verify(signedTransaction: signedTransaction)
    } catch {
      Self.logger.error(
        "Subscription verify POST failed: \(error.localizedDescription, privacy: .public)")
    }
  }

  // MARK: - Helpers

  private func checkVerification<T>(
    _ result: VerificationResult<T>
  ) throws -> T {
    switch result {
    case .verified(let value):
      return value
    case .unverified:
      throw DomainError.purchaseFailed("Transaction verification failed")
    }
  }

  private func entitlement(from transaction: Transaction) -> SubscriptionEntitlement {
    let tier = Self.tierMapping[transaction.productID] ?? .free
    let expiry = transaction.expirationDate ?? Date.distantFuture
    let isTrial = transaction.offerType == .introductory

    return SubscriptionEntitlement(
      tier: tier,
      expiryDate: expiry,
      isTrialPeriod: isTrial
    )
  }

  private func tierOrder(_ tier: SubscriptionTier) -> Int {
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
