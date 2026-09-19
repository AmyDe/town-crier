import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("SubscriptionViewModel")
@MainActor
struct SubscriptionViewModelTests {
  private func makeSUT() -> (
    SubscriptionViewModel, SpySubscriptionService, SpyAuthenticationService
  ) {
    let subscriptionSpy = SpySubscriptionService()
    let authSpy = SpyAuthenticationService()
    let sut = SubscriptionViewModel(
      subscriptionService: subscriptionSpy,
      authenticationService: authSpy,
      locale: Locale(identifier: "en_GB")
    )
    return (sut, subscriptionSpy, authSpy)
  }

  // MARK: - Initial state

  @Test func init_hasNoProducts() {
    let (sut, _, _) = makeSUT()
    #expect(sut.products.isEmpty)
  }

  @Test func init_isNotLoading() {
    let (sut, _, _) = makeSUT()
    #expect(!sut.isLoading)
  }

  @Test func init_hasNoError() {
    let (sut, _, _) = makeSUT()
    #expect(sut.error == nil)
  }

  @Test func init_hasNoEntitlement() {
    let (sut, _, _) = makeSUT()
    #expect(sut.currentEntitlement == nil)
  }

  @Test func init_isPurchasingIsFalse() {
    let (sut, _, _) = makeSUT()
    #expect(!sut.isPurchasing)
  }

  @Test func init_isRestoringIsFalse() {
    let (sut, _, _) = makeSUT()
    #expect(!sut.isRestoring)
  }

  // MARK: - Load products

  @Test func loadProducts_populatesProducts_onSuccess() async {
    let (sut, spy, _) = makeSUT()
    let expected: [SubscriptionProduct] = [.personal, .pro]
    spy.availableProductsResult = .success(expected)

    await sut.loadProducts()

    #expect(sut.products == expected)
    #expect(!sut.isLoading)
    #expect(sut.error == nil)
  }

  @Test func loadProducts_callsService() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.personal])

    await sut.loadProducts()

    #expect(spy.availableProductsCallCount == 1)
  }

  @Test func loadProducts_setsError_onFailure() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .failure(DomainError.networkUnavailable)

    await sut.loadProducts()

    #expect(sut.products.isEmpty)
    #expect(sut.error == .networkUnavailable)
  }

  @Test func loadProducts_alsoLoadsCurrentEntitlement() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.personal])
    spy.currentEntitlementResult = .personalActive

    await sut.loadProducts()

    #expect(sut.currentEntitlement == .personalActive)
    #expect(spy.currentEntitlementCallCount == 1)
  }

  // MARK: - Purchase

  @Test func purchase_setsEntitlement_onSuccess() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .success(.personalActive)

    await sut.purchase(productId: "uk.towncrierapp.personal.monthly")

    #expect(sut.currentEntitlement == .personalActive)
    #expect(!sut.isPurchasing)
    #expect(sut.error == nil)
  }

  @Test func purchase_callsServiceWithProductId() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .success(.personalActive)

    await sut.purchase(productId: "uk.towncrierapp.personal.monthly")

    #expect(spy.purchaseCalls == ["uk.towncrierapp.personal.monthly"])
  }

  @Test func purchase_setsError_onFailure() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .failure(DomainError.purchaseFailed("payment declined"))

    await sut.purchase(productId: "uk.towncrierapp.personal.monthly")

    #expect(sut.currentEntitlement == nil)
    #expect(sut.error == .purchaseFailed("payment declined"))
  }

  @Test func purchase_setsNilError_onCancellation() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .failure(DomainError.purchaseCancelled)

    await sut.purchase(productId: "uk.towncrierapp.personal.monthly")

    #expect(sut.error == nil)
    #expect(!sut.isPurchasing)
  }

  @Test func purchase_clearsError_beforeAttempt() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .failure(DomainError.purchaseFailed("fail"))
    await sut.purchase(productId: "id")
    #expect(sut.error != nil)

    spy.purchaseResult = .success(.personalActive)
    await sut.purchase(productId: "id")

    #expect(sut.error == nil)
  }

  // MARK: - Restore purchases

  @Test func restorePurchases_setsEntitlement_whenFound() async {
    let (sut, spy, _) = makeSUT()
    spy.restorePurchasesResult = .success(.proActive)

    await sut.restorePurchases()

    #expect(sut.currentEntitlement == .proActive)
    #expect(!sut.isRestoring)
  }

  @Test func restorePurchases_callsService() async {
    let (sut, spy, _) = makeSUT()
    spy.restorePurchasesResult = .success(nil)

    await sut.restorePurchases()

    #expect(spy.restorePurchasesCallCount == 1)
  }

  @Test func restorePurchases_leavesEntitlementNil_whenNoneFound() async {
    let (sut, spy, _) = makeSUT()
    spy.restorePurchasesResult = .success(nil)

    await sut.restorePurchases()

    #expect(sut.currentEntitlement == nil)
    #expect(sut.error == nil)
  }

  @Test func restorePurchases_setsError_onFailure() async {
    let (sut, spy, _) = makeSUT()
    spy.restorePurchasesResult = .failure(DomainError.restoreFailed("no account"))

    await sut.restorePurchases()

    #expect(sut.error == .restoreFailed("no account"))
  }

  // MARK: - Subscription disclosure

  @Test func subscriptionDisclosure_returnsTermsForProduct() {
    let (sut, _, _) = makeSUT()

    let disclosure = sut.subscriptionDisclosure(for: .personal)

    #expect(disclosure.contains("£1.99"))
    #expect(disclosure.contains("automatically renew"))
  }

  @Test func subscriptionDisclosure_mentionsTrial_whenAvailable() {
    let (sut, _, _) = makeSUT()

    let disclosure = sut.subscriptionDisclosure(for: .personal)

    #expect(disclosure.contains("7-day free trial"))
  }

  @Test func subscriptionDisclosure_omitsTrial_whenNotAvailable() {
    let (sut, _, _) = makeSUT()

    let disclosure = sut.subscriptionDisclosure(for: .pro)

    #expect(!disclosure.contains("free trial"))
  }

  // MARK: - Legal documents

  @Test func init_hasNoPresentedLegalDocument() {
    let (sut, _, _) = makeSUT()
    #expect(sut.presentedLegalDocument == nil)
  }

  @Test func showLegalDocument_presentsPrivacyPolicy() {
    let (sut, _, _) = makeSUT()

    sut.showLegalDocument(.privacyPolicy)

    #expect(sut.presentedLegalDocument == .privacyPolicy)
  }

  @Test func showLegalDocument_presentsTermsOfService() {
    let (sut, _, _) = makeSUT()

    sut.showLegalDocument(.termsOfService)

    #expect(sut.presentedLegalDocument == .termsOfService)
  }

  // MARK: - Computed properties

  @Test func isSubscribed_returnsFalse_whenNoEntitlement() {
    let (sut, _, _) = makeSUT()
    #expect(!sut.isSubscribed)
  }

  @Test func isSubscribed_returnsTrue_whenHasActiveEntitlement() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .success(.personalActive)
    await sut.purchase(productId: "id")

    #expect(sut.isSubscribed)
  }

  // MARK: - Post-purchase token refresh

  @Test func purchase_refreshesAuthSession_afterSuccess() async {
    let (sut, subscriptionSpy, authSpy) = makeSUT()
    subscriptionSpy.purchaseResult = .success(.personalActive)
    authSpy.refreshSessionResult = .success(.personal)

    await sut.purchase(productId: "uk.towncrierapp.personal.monthly")

    #expect(authSpy.refreshSessionCallCount == 1)
  }

  @Test func purchase_doesNotRefreshSession_onFailure() async {
    let (sut, subscriptionSpy, authSpy) = makeSUT()
    subscriptionSpy.purchaseResult = .failure(DomainError.purchaseFailed("declined"))

    await sut.purchase(productId: "uk.towncrierapp.personal.monthly")

    #expect(authSpy.refreshSessionCallCount == 0)
  }

  @Test func purchase_doesNotRefreshSession_onCancellation() async {
    let (sut, subscriptionSpy, authSpy) = makeSUT()
    subscriptionSpy.purchaseResult = .failure(DomainError.purchaseCancelled)

    await sut.purchase(productId: "uk.towncrierapp.personal.monthly")

    #expect(authSpy.refreshSessionCallCount == 0)
  }

  @Test func purchase_succeedsEvenWhenTokenRefreshFails() async {
    let (sut, subscriptionSpy, authSpy) = makeSUT()
    subscriptionSpy.purchaseResult = .success(.personalActive)
    authSpy.refreshSessionResult = .failure(DomainError.sessionExpired)

    await sut.purchase(productId: "uk.towncrierapp.personal.monthly")

    #expect(sut.currentEntitlement == .personalActive)
    #expect(sut.error == nil)
  }

  // MARK: - Post-restore token refresh

  @Test func restorePurchases_refreshesAuthSession_afterSuccess() async {
    let (sut, subscriptionSpy, authSpy) = makeSUT()
    subscriptionSpy.restorePurchasesResult = .success(.proActive)
    authSpy.refreshSessionResult = .success(.pro)

    await sut.restorePurchases()

    #expect(authSpy.refreshSessionCallCount == 1)
  }

  @Test func restorePurchases_doesNotRefreshSession_whenNoneFound() async {
    let (sut, subscriptionSpy, authSpy) = makeSUT()
    subscriptionSpy.restorePurchasesResult = .success(nil)

    await sut.restorePurchases()

    #expect(authSpy.refreshSessionCallCount == 0)
  }

  @Test func restorePurchases_succeedsEvenWhenTokenRefreshFails() async {
    let (sut, subscriptionSpy, authSpy) = makeSUT()
    subscriptionSpy.restorePurchasesResult = .success(.proActive)
    authSpy.refreshSessionResult = .failure(DomainError.sessionExpired)

    await sut.restorePurchases()

    #expect(sut.currentEntitlement == .proActive)
    #expect(sut.error == nil)
  }

  // MARK: - Product grouping

  @Test func personalProducts_containsOnlyPersonalTier() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])

    await sut.loadProducts()

    #expect(sut.personalProducts == [.personal])
  }

  @Test func proOptions_areProTierOrderedMonthlyAnnualLifetime() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.proLifetime, .personal, .proAnnual, .pro])

    await sut.loadProducts()

    #expect(sut.proOptions == [.pro, .proAnnual, .proLifetime])
  }

  // MARK: - Selected Pro period

  @Test func init_selectedProPeriodIsAnnual() {
    let (sut, _, _) = makeSUT()
    #expect(sut.selectedProPeriod == .annual)
  }

  @Test func loadProducts_defaultsSelectedPeriodToAnnual_whenNoEntitlement() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])

    await sut.loadProducts()

    #expect(sut.selectedProPeriod == .annual)
  }

  @Test func loadProducts_defaultsSelectedPeriodToAnnual_whenEntitlementIsPersonal() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])
    spy.currentEntitlementResult = .personalActive

    await sut.loadProducts()

    #expect(sut.selectedProPeriod == .annual)
  }

  @Test func loadProducts_selectsMonthly_whenEntitlementIsProMonthly() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])
    spy.currentEntitlementResult = .proMonthlyActive

    await sut.loadProducts()

    #expect(sut.selectedProPeriod == .monthly)
  }

  @Test func loadProducts_selectsAnnual_whenEntitlementIsProAnnual() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])
    spy.currentEntitlementResult = .proAnnualActive

    await sut.loadProducts()

    #expect(sut.selectedProPeriod == .annual)
  }

  @Test func loadProducts_selectsLifetime_whenEntitlementIsLifetime() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])
    spy.currentEntitlementResult = .proLifetime

    await sut.loadProducts()

    #expect(sut.selectedProPeriod == .lifetime)
  }

  @Test func loadProducts_fallsBackToFirstOption_whenSelectedPeriodIsNotOffered() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro])

    await sut.loadProducts()

    #expect(sut.selectedProPeriod == .monthly)
  }

  @Test func selectedProProduct_isTheOptionForTheSelectedPeriod() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])
    await sut.loadProducts()

    sut.selectedProPeriod = .lifetime

    #expect(sut.selectedProProduct == .proLifetime)
  }

  // MARK: - Annual savings

  @Test func annualSavingsPercent_is50_for2999Against499() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.pro, .proAnnual])
    await sut.loadProducts()

    #expect(sut.annualSavingsPercent == 50)
  }

  @Test func annualSavingsPercent_isNil_whenAnnualProductIsMissing() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.pro, .proLifetime])
    await sut.loadProducts()

    #expect(sut.annualSavingsPercent == nil)
  }

  @Test func annualSavingsPercent_isNil_whenMonthlyProductIsMissing() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.proAnnual])
    await sut.loadProducts()

    #expect(sut.annualSavingsPercent == nil)
  }

  @Test func annualSavingsPercent_isNil_whenSavingRoundsBelowOnePercent() async {
    let (sut, spy, _) = makeSUT()
    let dearAnnual = SubscriptionProduct(
      id: "uk.towncrierapp.pro.annual", displayName: "Pro Annual", displayPrice: "£59.88",
      tier: .pro, period: .annual, price: 59.88
    )
    spy.availableProductsResult = .success([.pro, dearAnnual])
    await sut.loadProducts()

    #expect(sut.annualSavingsPercent == nil)
  }

  @Test func annualMonthlyEquivalent_is250_for2999() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.pro, .proAnnual])
    await sut.loadProducts()

    #expect(sut.annualMonthlyEquivalent == "£2.50")
  }

  @Test func annualMonthlyEquivalent_isNil_whenAnnualProductIsMissing() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.pro])
    await sut.loadProducts()

    #expect(sut.annualMonthlyEquivalent == nil)
  }

  @Test func annualSubLine_readsWorksOutAtMonthlyEquivalent() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.pro, .proAnnual])
    await sut.loadProducts()

    #expect(sut.annualSubLine == "Works out at £2.50 a month")
  }

  @Test func annualSavingsBadge_readsSavePercent() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.pro, .proAnnual])
    await sut.loadProducts()

    #expect(sut.annualSavingsBadge == "Save 50%")
  }

  @Test func annualSavingsBadge_isNil_whenThereIsNoSaving() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.pro])
    await sut.loadProducts()

    #expect(sut.annualSavingsBadge == nil)
  }

  // MARK: - Price and button copy

  @Test func priceLine_monthly_appendsSlashMonth() {
    let (sut, _, _) = makeSUT()
    #expect(sut.priceLine(for: .pro) == "£4.99/month")
  }

  @Test func priceLine_annual_appendsSlashYear() {
    let (sut, _, _) = makeSUT()
    #expect(sut.priceLine(for: .proAnnual) == "£29.99/year")
  }

  @Test func priceLine_lifetime_appendsOnce() {
    let (sut, _, _) = makeSUT()
    #expect(sut.priceLine(for: .proLifetime) == "£69.99 once")
  }

  @Test func purchaseButtonTitle_monthly_isSubscribe() {
    let (sut, _, _) = makeSUT()
    #expect(sut.purchaseButtonTitle(for: .pro) == "Subscribe")
  }

  @Test func purchaseButtonTitle_monthlyWithTrial_isStartFreeTrial() {
    let (sut, _, _) = makeSUT()
    #expect(sut.purchaseButtonTitle(for: .personal) == "Start Free Trial")
  }

  @Test func purchaseButtonTitle_annual_isSubscribeYearly() {
    let (sut, _, _) = makeSUT()
    #expect(sut.purchaseButtonTitle(for: .proAnnual) == "Subscribe yearly")
  }

  @Test func purchaseButtonTitle_lifetime_isBuyLifetime() {
    let (sut, _, _) = makeSUT()
    #expect(sut.purchaseButtonTitle(for: .proLifetime) == "Buy lifetime")
  }

  @Test func periodPickerTitles_areMonthlyYearlyLifetime() {
    #expect(SubscriptionPeriod.monthly.pickerTitle == "Monthly")
    #expect(SubscriptionPeriod.annual.pickerTitle == "Yearly")
    #expect(SubscriptionPeriod.lifetime.pickerTitle == "Lifetime")
  }

  // MARK: - Period disclosures

  @Test func subscriptionDisclosure_annual_returnsAnnualText() {
    let (sut, _, _) = makeSUT()

    #expect(
      sut.subscriptionDisclosure(for: .proAnnual)
        == "Your subscription renews automatically at £29.99/year unless you cancel at least "
        + "24 hours before the end of the current period. You can manage or cancel it in your "
        + "App Store settings."
    )
  }

  @Test func subscriptionDisclosure_lifetime_returnsLifetimeText() {
    let (sut, _, _) = makeSUT()

    #expect(
      sut.subscriptionDisclosure(for: .proLifetime)
        == "One payment of £69.99. This is not a subscription and nothing renews. "
        + "Pro stays on your account for as long as Town Crier runs."
    )
  }

  // MARK: - Current product

  @Test func isCurrentProduct_isTrueOnlyForTheEntitlementsProductId() async {
    let (sut, spy, _) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])
    spy.currentEntitlementResult = .proAnnualActive
    await sut.loadProducts()

    #expect(sut.isCurrentProduct(.proAnnual))
    #expect(!sut.isCurrentProduct(.pro))
    #expect(!sut.isCurrentProduct(.proLifetime))
    #expect(!sut.isCurrentProduct(.personal))
  }

  @Test func isCurrentProduct_isFalse_whenThereIsNoEntitlement() {
    let (sut, _, _) = makeSUT()
    #expect(!sut.isCurrentProduct(.pro))
  }

  // MARK: - Purchase buttons

  @Test func showsPurchaseButtons_isTrue_withoutLifetimeEntitlement() async {
    let (sut, spy, _) = makeSUT()
    spy.currentEntitlementResult = .proMonthlyActive
    await sut.loadProducts()

    #expect(sut.showsPurchaseButtons)
  }

  @Test func showsPurchaseButtons_isFalse_withLifetimeEntitlement() async {
    let (sut, spy, _) = makeSUT()
    spy.currentEntitlementResult = .proLifetime
    await sut.loadProducts()

    #expect(!sut.showsPurchaseButtons)
  }

  @Test func showsPurchaseButtons_isFalse_afterPurchasingLifetime() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .success(.proLifetime)

    await sut.purchase(productId: "uk.towncrierapp.pro.lifetime")

    #expect(!sut.showsPurchaseButtons)
  }

  // MARK: - Cancel-old-subscription prompt

  @Test func init_cancelPromptAndManageSheetAreNotPresented() {
    let (sut, _, _) = makeSUT()
    #expect(!sut.isCancelSubscriptionPromptPresented)
    #expect(!sut.isManageSubscriptionsPresented)
  }

  @Test func purchase_lifetimeWithRenewingSubscription_presentsCancelPrompt() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .success(.proLifetime)
    spy.hasActiveAutoRenewingSubscriptionResult = true

    await sut.purchase(productId: "uk.towncrierapp.pro.lifetime")

    #expect(sut.isCancelSubscriptionPromptPresented)
    #expect(spy.hasActiveAutoRenewingSubscriptionCallCount == 1)
  }

  @Test func purchase_lifetimeWithoutRenewingSubscription_doesNotPresentCancelPrompt() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .success(.proLifetime)
    spy.hasActiveAutoRenewingSubscriptionResult = false

    await sut.purchase(productId: "uk.towncrierapp.pro.lifetime")

    #expect(!sut.isCancelSubscriptionPromptPresented)
    #expect(spy.hasActiveAutoRenewingSubscriptionCallCount == 1)
  }

  @Test func purchase_monthly_neverPresentsCancelPrompt() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .success(.proMonthlyActive)
    spy.hasActiveAutoRenewingSubscriptionResult = true

    await sut.purchase(productId: "uk.towncrierapp.pro.monthly")

    #expect(!sut.isCancelSubscriptionPromptPresented)
    #expect(spy.hasActiveAutoRenewingSubscriptionCallCount == 0)
  }

  @Test func purchase_annual_neverPresentsCancelPrompt() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .success(.proAnnualActive)
    spy.hasActiveAutoRenewingSubscriptionResult = true

    await sut.purchase(productId: "uk.towncrierapp.pro.annual")

    #expect(!sut.isCancelSubscriptionPromptPresented)
    #expect(spy.hasActiveAutoRenewingSubscriptionCallCount == 0)
  }

  @Test func purchase_lifetimeFailure_doesNotPresentCancelPrompt() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .failure(DomainError.purchaseFailed("declined"))
    spy.hasActiveAutoRenewingSubscriptionResult = true

    await sut.purchase(productId: "uk.towncrierapp.pro.lifetime")

    #expect(!sut.isCancelSubscriptionPromptPresented)
    #expect(spy.hasActiveAutoRenewingSubscriptionCallCount == 0)
  }

  @Test func showManageSubscriptions_presentsTheManageSheet() {
    let (sut, _, _) = makeSUT()

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
