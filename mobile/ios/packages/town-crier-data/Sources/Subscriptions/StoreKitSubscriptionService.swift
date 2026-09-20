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
      let transaction = try checkVerification(verification)
      await transaction.finish()
      // Tell the backend about the purchase so tier-gated API requests see
      // the new tier (ADR 0010). Best-effort — never fails the purchase.
      await reportPurchase(signedTransaction: verification.jwsRepresentation)
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
    var latestEntitlement: SubscriptionEntitlement?
    var signedTransactions: [String] = []

    for await result in Transaction.currentEntitlements {
      // Unverified entitlements are sent too: the server is the authority on a restore.
      signedTransactions.append(result.jwsRepresentation)

      if let transaction = try? checkVerification(result) {
        let ent = Self.entitlement(from: transaction)
        if ent.isActive {
          latestEntitlement = Self.preferredEntitlement(current: latestEntitlement, candidate: ent)
        }
      }
    }

    try await reportRestore(signedTransactions: signedTransactions)

    return latestEntitlement
  }

  public func currentEntitlement() async -> SubscriptionEntitlement? {
    try? await restorePurchases()
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

  /// POSTs the JWS list from `Transaction.currentEntitlements` to the Town
  /// Crier backend via the injected ``SubscriptionVerificationService`` so the
  /// server re-verifies the restore and updates Cosmos (ADR 0010 — Restore
  /// Purchases).
  ///
  /// A restore is an explicit user action initiated from the "Restore
  /// Purchases" control, so — unlike ``reportPurchase(signedTransaction:)`` —
  /// a verification rejection (e.g. an HTTP 401 for a tampered JWS) is
  /// rethrown so the UI can tell the user. An empty list is a no-op: there is
  /// nothing to verify, and the backend already treats the absence of active
  /// transactions as the `Free` tier. No-op too when no service is injected.
  func reportRestore(signedTransactions: [String]) async throws {
    guard let verificationService, !signedTransactions.isEmpty else { return }
    _ = try await verificationService.verifyRestore(signedTransactions: signedTransactions)
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
