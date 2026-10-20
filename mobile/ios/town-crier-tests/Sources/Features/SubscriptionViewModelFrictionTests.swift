import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// A failed purchase fires `onPurchaseFailed` to suppress the review prompt; a
/// cancelled or successful purchase must not.
@Suite("SubscriptionViewModel — purchase friction")
@MainActor
struct SubscriptionViewModelFrictionTests {
  private func makeSUT(
    purchaseResult: Result<SubscriptionEntitlement, Error>
  ) -> SubscriptionViewModel {
    let service = SpySubscriptionService()
    service.purchaseResult = purchaseResult
    return SubscriptionViewModel(
      subscriptionService: service,
      authenticationService: SpyAuthenticationService()
    )
  }

  @Test("a failed purchase fires onPurchaseFailed")
  func failedPurchaseFiresFriction() async {
    let sut = makeSUT(purchaseResult: .failure(DomainError.networkUnavailable))
    var frictionCount = 0
    sut.onPurchaseFailed = { frictionCount += 1 }

    await sut.purchase(productId: "premium")

    #expect(frictionCount == 1)
  }

  @Test("a cancelled purchase does not fire onPurchaseFailed")
  func cancelledPurchaseDoesNotFireFriction() async {
    let sut = makeSUT(purchaseResult: .failure(DomainError.purchaseCancelled))
    var frictionCount = 0
    sut.onPurchaseFailed = { frictionCount += 1 }

    await sut.purchase(productId: "premium")

    #expect(frictionCount == 0)
  }

  @Test("a successful purchase does not fire onPurchaseFailed")
  func successfulPurchaseDoesNotFireFriction() async {
    let sut = makeSUT(purchaseResult: .success(.personalActive))
    var frictionCount = 0
    sut.onPurchaseFailed = { frictionCount += 1 }

    await sut.purchase(productId: "premium")

    #expect(frictionCount == 0)
  }
}
