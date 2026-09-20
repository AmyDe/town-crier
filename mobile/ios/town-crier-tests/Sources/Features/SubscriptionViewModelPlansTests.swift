import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("SubscriptionViewModel — Pro plans")
@MainActor
struct SubscriptionViewModelPlansTests {
  private func makeSUT() -> (SubscriptionViewModel, SpySubscriptionService) {
    let subscriptionSpy = SpySubscriptionService()
    let sut = SubscriptionViewModel(
      subscriptionService: subscriptionSpy,
      authenticationService: SpyAuthenticationService(),
      locale: Locale(identifier: "en_GB")
    )
    return (sut, subscriptionSpy)
  }

  @Test func personalProducts_containsOnlyPersonalTier() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])

    await sut.loadProducts()

    #expect(sut.personalProducts == [.personal])
  }

  @Test func proOptions_areProTierOrderedMonthlyAnnualLifetime() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.proLifetime, .personal, .proAnnual, .pro])

    await sut.loadProducts()

    #expect(sut.proOptions == [.pro, .proAnnual, .proLifetime])
  }

  @Test func init_selectedProPeriodIsAnnual() {
    let (sut, _) = makeSUT()
    #expect(sut.selectedProPeriod == .annual)
  }

  @Test func loadProducts_defaultsSelectedPeriodToAnnual_whenNoEntitlement() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])

    await sut.loadProducts()

    #expect(sut.selectedProPeriod == .annual)
  }

  @Test func loadProducts_defaultsSelectedPeriodToAnnual_whenEntitlementIsPersonal() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])
    spy.currentEntitlementResult = .personalActive

    await sut.loadProducts()

    #expect(sut.selectedProPeriod == .annual)
  }

  @Test func loadProducts_selectsMonthly_whenEntitlementIsProMonthly() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])
    spy.currentEntitlementResult = .proMonthlyActive

    await sut.loadProducts()

    #expect(sut.selectedProPeriod == .monthly)
  }

  @Test func loadProducts_selectsAnnual_whenEntitlementIsProAnnual() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])
    spy.currentEntitlementResult = .proAnnualActive

    await sut.loadProducts()

    #expect(sut.selectedProPeriod == .annual)
  }

  @Test func loadProducts_selectsLifetime_whenEntitlementIsLifetime() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])
    spy.currentEntitlementResult = .proLifetime

    await sut.loadProducts()

    #expect(sut.selectedProPeriod == .lifetime)
  }

  @Test func loadProducts_fallsBackToFirstOption_whenSelectedPeriodIsNotOffered() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro])

    await sut.loadProducts()

    #expect(sut.selectedProPeriod == .monthly)
  }

  @Test func selectedProProduct_isTheOptionForTheSelectedPeriod() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])
    await sut.loadProducts()

    sut.selectedProPeriod = .lifetime

    #expect(sut.selectedProProduct == .proLifetime)
  }

  @Test func annualSavingsPercent_is50_for2999Against499() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.pro, .proAnnual])
    await sut.loadProducts()

    #expect(sut.annualSavingsPercent == 50)
  }

  @Test func annualSavingsPercent_isNil_whenAnnualProductIsMissing() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.pro, .proLifetime])
    await sut.loadProducts()

    #expect(sut.annualSavingsPercent == nil)
  }

  @Test func annualSavingsPercent_isNil_whenMonthlyProductIsMissing() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.proAnnual])
    await sut.loadProducts()

    #expect(sut.annualSavingsPercent == nil)
  }

  @Test func annualSavingsPercent_isNil_whenSavingRoundsBelowOnePercent() async {
    let (sut, spy) = makeSUT()
    let dearAnnual = SubscriptionProduct(
      id: "uk.towncrierapp.pro.annual",
      displayName: "Pro Annual",
      displayPrice: "£59.88",
      tier: .pro,
      period: .annual,
      price: 59.88
    )
    spy.availableProductsResult = .success([.pro, dearAnnual])
    await sut.loadProducts()

    #expect(sut.annualSavingsPercent == nil)
  }

  @Test func annualMonthlyEquivalent_is250_for2999() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.pro, .proAnnual])
    await sut.loadProducts()

    #expect(sut.annualMonthlyEquivalent == "£2.50")
  }

  @Test func annualMonthlyEquivalent_isNil_whenAnnualProductIsMissing() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.pro])
    await sut.loadProducts()

    #expect(sut.annualMonthlyEquivalent == nil)
  }

  @Test func annualSubLine_readsWorksOutAtMonthlyEquivalent() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.pro, .proAnnual])
    await sut.loadProducts()

    #expect(sut.annualSubLine == "Works out at £2.50 a month")
  }

  @Test func annualSavingsBadge_readsSavePercent() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.pro, .proAnnual])
    await sut.loadProducts()

    #expect(sut.annualSavingsBadge == "Save 50%")
  }

  @Test func annualSavingsBadge_isNil_whenThereIsNoSaving() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.pro])
    await sut.loadProducts()

    #expect(sut.annualSavingsBadge == nil)
  }

  @Test func priceLine_monthly_appendsSlashMonth() {
    let (sut, _) = makeSUT()
    #expect(sut.priceLine(for: .pro) == "£4.99/month")
  }

  @Test func priceLine_annual_appendsSlashYear() {
    let (sut, _) = makeSUT()
    #expect(sut.priceLine(for: .proAnnual) == "£29.99/year")
  }

  @Test func priceLine_lifetime_appendsOnce() {
    let (sut, _) = makeSUT()
    #expect(sut.priceLine(for: .proLifetime) == "£69.99 once")
  }

  @Test func purchaseButtonTitle_monthly_isSubscribe() {
    let (sut, _) = makeSUT()
    #expect(sut.purchaseButtonTitle(for: .pro) == "Subscribe")
  }

  @Test func purchaseButtonTitle_monthlyWithTrial_isStartFreeTrial() {
    let (sut, _) = makeSUT()
    #expect(sut.purchaseButtonTitle(for: .personal) == "Start Free Trial")
  }

  @Test func purchaseButtonTitle_annual_isSubscribeYearly() {
    let (sut, _) = makeSUT()
    #expect(sut.purchaseButtonTitle(for: .proAnnual) == "Subscribe yearly")
  }

  @Test func purchaseButtonTitle_lifetime_isBuyLifetime() {
    let (sut, _) = makeSUT()
    #expect(sut.purchaseButtonTitle(for: .proLifetime) == "Buy lifetime")
  }

  @Test func periodPickerTitles_areMonthlyYearlyLifetime() {
    #expect(SubscriptionPeriod.monthly.pickerTitle == "Monthly")
    #expect(SubscriptionPeriod.annual.pickerTitle == "Yearly")
    #expect(SubscriptionPeriod.lifetime.pickerTitle == "Lifetime")
  }

  @Test func subscriptionDisclosure_annual_returnsAnnualText() {
    let (sut, _) = makeSUT()

    #expect(
      sut.subscriptionDisclosure(for: .proAnnual)
        == "Your subscription renews automatically at £29.99/year unless you cancel at least "
        + "24 hours before the end of the current period. You can manage or cancel it in your "
        + "App Store settings."
    )
  }

  @Test func subscriptionDisclosure_lifetime_returnsLifetimeText() {
    let (sut, _) = makeSUT()

    #expect(
      sut.subscriptionDisclosure(for: .proLifetime)
        == "One payment of £69.99. This is not a subscription and nothing renews. "
        + "Pro stays on your account for as long as Town Crier runs."
    )
  }

  @Test func isCurrentProduct_isTrueOnlyForTheEntitlementsProductId() async {
    let (sut, spy) = makeSUT()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])
    spy.currentEntitlementResult = .proAnnualActive
    await sut.loadProducts()

    #expect(sut.isCurrentProduct(.proAnnual))
    #expect(!sut.isCurrentProduct(.pro))
    #expect(!sut.isCurrentProduct(.proLifetime))
    #expect(!sut.isCurrentProduct(.personal))
  }

  @Test func isCurrentProduct_isFalse_whenThereIsNoEntitlement() {
    let (sut, _) = makeSUT()
    #expect(!sut.isCurrentProduct(.pro))
  }

  @Test func showsPurchaseButtons_isTrue_withoutLifetimeEntitlement() async {
    let (sut, spy) = makeSUT()
    spy.currentEntitlementResult = .proMonthlyActive
    await sut.loadProducts()

    #expect(sut.showsPurchaseButtons)
  }

  @Test func showsPurchaseButtons_isFalse_withLifetimeEntitlement() async {
    let (sut, spy) = makeSUT()
    spy.currentEntitlementResult = .proLifetime
    await sut.loadProducts()

    #expect(!sut.showsPurchaseButtons)
  }

  @Test func showsPurchaseButtons_isFalse_afterPurchasingLifetime() async {
    let (sut, spy) = makeSUT()
    spy.purchaseResult = .success(.proLifetime)

    await sut.purchase(productId: "uk.towncrierapp.pro.lifetime")

    #expect(!sut.showsPurchaseButtons)
  }
}
