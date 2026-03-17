import Foundation
import TownCrierDomain

final class SpySubscriptionService: SubscriptionService, @unchecked Sendable {
    private(set) var availableProductsCallCount = 0
    var availableProductsResult: Result<[SubscriptionProduct], Error> = .success([])

    func availableProducts() async throws -> [SubscriptionProduct] {
        availableProductsCallCount += 1
        return try availableProductsResult.get()
    }

    private(set) var purchaseCalls: [String] = []
    var purchaseResult: Result<SubscriptionEntitlement, Error> = .success(.personalActive)

    func purchase(_ productId: String) async throws -> SubscriptionEntitlement {
        purchaseCalls.append(productId)
        return try purchaseResult.get()
    }

    private(set) var restorePurchasesCallCount = 0
    var restorePurchasesResult: Result<SubscriptionEntitlement?, Error> = .success(nil)

    func restorePurchases() async throws -> SubscriptionEntitlement? {
        restorePurchasesCallCount += 1
        return try restorePurchasesResult.get()
    }

    private(set) var currentEntitlementCallCount = 0
    var currentEntitlementResult: SubscriptionEntitlement?

    func currentEntitlement() async -> SubscriptionEntitlement? {
        currentEntitlementCallCount += 1
        return currentEntitlementResult
    }
}
