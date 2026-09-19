import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("SubscriptionViewModel — cancel-old-subscription prompt")
@MainActor
struct SubscriptionViewModelCancelPromptTests {
  private func makeSUT() -> (SubscriptionViewModel, SpySubscriptionService) {
    let subscriptionSpy = SpySubscriptionService()
    let sut = SubscriptionViewModel(
      subscriptionService: subscriptionSpy,
      authenticationService: SpyAuthenticationService(),
      locale: Locale(identifier: "en_GB")
    )
    return (sut, subscriptionSpy)
  }

  // MARK: - Cancel-old-subscription prompt

  @Test func init_cancelPromptAndManageSheetAreNotPresented() {
    let (sut, _) = makeSUT()
    #expect(!sut.isCancelSubscriptionPromptPresented)
    #expect(!sut.isManageSubscriptionsPresented)
  }

  @Test func purchase_lifetimeWithRenewingSubscription_presentsCancelPrompt() async {
    let (sut, spy) = makeSUT()
    spy.purchaseResult = .success(.proLifetime)
    spy.hasActiveAutoRenewingSubscriptionResult = true

    await sut.purchase(productId: "uk.towncrierapp.pro.lifetime")

    #expect(sut.isCancelSubscriptionPromptPresented)
    #expect(spy.autoRenewingCheckCallCount == 1)
  }

  @Test func purchase_lifetimeWithoutRenewingSubscription_doesNotPresentCancelPrompt() async {
    let (sut, spy) = makeSUT()
    spy.purchaseResult = .success(.proLifetime)
    spy.hasActiveAutoRenewingSubscriptionResult = false

    await sut.purchase(productId: "uk.towncrierapp.pro.lifetime")

    #expect(!sut.isCancelSubscriptionPromptPresented)
    #expect(spy.autoRenewingCheckCallCount == 1)
  }

  @Test func purchase_monthly_neverPresentsCancelPrompt() async {
    let (sut, spy) = makeSUT()
    spy.purchaseResult = .success(.proMonthlyActive)
    spy.hasActiveAutoRenewingSubscriptionResult = true

    await sut.purchase(productId: "uk.towncrierapp.pro.monthly")

    #expect(!sut.isCancelSubscriptionPromptPresented)
    #expect(spy.autoRenewingCheckCallCount == 0)
  }

  @Test func purchase_annual_neverPresentsCancelPrompt() async {
    let (sut, spy) = makeSUT()
    spy.purchaseResult = .success(.proAnnualActive)
    spy.hasActiveAutoRenewingSubscriptionResult = true

    await sut.purchase(productId: "uk.towncrierapp.pro.annual")

    #expect(!sut.isCancelSubscriptionPromptPresented)
    #expect(spy.autoRenewingCheckCallCount == 0)
  }

  @Test func purchase_lifetimeFailure_doesNotPresentCancelPrompt() async {
    let (sut, spy) = makeSUT()
    spy.purchaseResult = .failure(DomainError.purchaseFailed("declined"))
    spy.hasActiveAutoRenewingSubscriptionResult = true

    await sut.purchase(productId: "uk.towncrierapp.pro.lifetime")

    #expect(!sut.isCancelSubscriptionPromptPresented)
    #expect(spy.autoRenewingCheckCallCount == 0)
  }

  @Test func showManageSubscriptions_presentsTheManageSheet() {
    let (sut, _) = makeSUT()

    sut.showManageSubscriptions()

    #expect(sut.isManageSubscriptionsPresented)
  }

  @Test func cancelPromptCopy_matchesTheApprovedText() {
    #expect(SubscriptionViewModel.cancelPromptTitle == "Cancel your old subscription")
    #expect(
      SubscriptionViewModel.cancelPromptMessage
        == "You now have lifetime Pro, but your subscription will still renew. "
        + "Apple doesn't cancel it for you, so cancel it now or you'll pay twice."
    )
    #expect(SubscriptionViewModel.cancelPromptManageButtonTitle == "Manage subscription")
    #expect(SubscriptionViewModel.cancelPromptDismissButtonTitle == "Not now")
  }
}
