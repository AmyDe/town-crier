import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// A purchase or restore whose Apple ID transaction already belongs to a
/// different Town Crier account (tc-k42ce.2, GH#1165). The single-owner rule
/// (`requireOwnedBy`, `api-go/internal/subscriptions/handler.go`) is correct
/// and unchanged -- this covers the app surfacing that verdict instead of
/// failing silently.
@Suite("SubscriptionViewModel — subscription claimed by another account")
@MainActor
struct SubscriptionViewModelTransactionClaimedTests {
  private func makeSUT() -> (SubscriptionViewModel, SpySubscriptionService) {
    let subscriptionSpy = SpySubscriptionService()
    let sut = SubscriptionViewModel(
      subscriptionService: subscriptionSpy,
      authenticationService: SpyAuthenticationService()
    )
    return (sut, subscriptionSpy)
  }

  @Test func init_transactionClaimedAlertIsNotPresented() {
    let (sut, _) = makeSUT()
    #expect(!sut.isTransactionClaimedAlertPresented)
  }

  // MARK: - Purchase

  @Test func purchase_transactionAlreadyClaimed_presentsAlert() async {
    let (sut, spy) = makeSUT()
    spy.purchaseResult = .failure(DomainError.transactionAlreadyClaimed)

    await sut.purchase(productId: "uk.towncrierapp.pro.annual")

    #expect(sut.isTransactionClaimedAlertPresented)
  }

  @Test func purchase_transactionAlreadyClaimed_doesNotSetEntitlement() async {
    let (sut, spy) = makeSUT()
    spy.purchaseResult = .failure(DomainError.transactionAlreadyClaimed)

    await sut.purchase(productId: "uk.towncrierapp.pro.annual")

    #expect(sut.currentEntitlement == nil)
  }

  @Test func purchase_transactionAlreadyClaimed_doesNotSetGenericError() async {
    let (sut, spy) = makeSUT()
    spy.purchaseResult = .failure(DomainError.transactionAlreadyClaimed)

    await sut.purchase(productId: "uk.towncrierapp.pro.annual")

    #expect(sut.error == nil)
  }

  @Test func purchase_transactionAlreadyClaimed_firesOnPurchaseFailed() async {
    let (sut, spy) = makeSUT()
    spy.purchaseResult = .failure(DomainError.transactionAlreadyClaimed)
    var frictionCount = 0
    sut.onPurchaseFailed = { frictionCount += 1 }

    await sut.purchase(productId: "uk.towncrierapp.pro.annual")

    #expect(frictionCount == 1)
  }

  // MARK: - Restore

  @Test func restorePurchases_transactionAlreadyClaimed_presentsAlert() async {
    let (sut, spy) = makeSUT()
    spy.restorePurchasesResult = .failure(DomainError.transactionAlreadyClaimed)

    await sut.restorePurchases()

    #expect(sut.isTransactionClaimedAlertPresented)
  }

  @Test func restorePurchases_transactionAlreadyClaimed_doesNotSetEntitlement() async {
    let (sut, spy) = makeSUT()
    spy.restorePurchasesResult = .failure(DomainError.transactionAlreadyClaimed)

    await sut.restorePurchases()

    #expect(sut.currentEntitlement == nil)
  }

  @Test func restorePurchases_transactionAlreadyClaimed_doesNotSetGenericError() async {
    let (sut, spy) = makeSUT()
    spy.restorePurchasesResult = .failure(DomainError.transactionAlreadyClaimed)

    await sut.restorePurchases()

    #expect(sut.error == nil)
  }

  // MARK: - Alert copy

  @Test func alertCopy_matchesTheApprovedText() {
    #expect(
      SubscriptionViewModel.transactionClaimedAlertTitle
        == "This subscription is on another account")
    #expect(
      SubscriptionViewModel.transactionClaimedAlertMessage
        == "The subscription on this Apple ID is already linked to a different Town Crier "
        + "account. Sign in with that account to use it, or get in touch and we'll sort it out.")
    #expect(SubscriptionViewModel.transactionClaimedAlertButtonTitle == "OK")
  }
}
