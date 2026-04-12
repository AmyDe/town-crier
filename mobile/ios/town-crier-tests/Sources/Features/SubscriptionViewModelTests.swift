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
      authenticationService: authSpy
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

    await sut.purchase(productId: "uk.co.towncrier.personal.monthly")

    #expect(sut.currentEntitlement == .personalActive)
    #expect(!sut.isPurchasing)
    #expect(sut.error == nil)
  }

  @Test func purchase_callsServiceWithProductId() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .success(.personalActive)

    await sut.purchase(productId: "uk.co.towncrier.personal.monthly")

    #expect(spy.purchaseCalls == ["uk.co.towncrier.personal.monthly"])
  }

  @Test func purchase_setsError_onFailure() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .failure(DomainError.purchaseFailed("payment declined"))

    await sut.purchase(productId: "uk.co.towncrier.personal.monthly")

    #expect(sut.currentEntitlement == nil)
    #expect(sut.error == .purchaseFailed("payment declined"))
  }

  @Test func purchase_setsNilError_onCancellation() async {
    let (sut, spy, _) = makeSUT()
    spy.purchaseResult = .failure(DomainError.purchaseCancelled)

    await sut.purchase(productId: "uk.co.towncrier.personal.monthly")

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

    await sut.purchase(productId: "uk.co.towncrier.personal.monthly")

    #expect(authSpy.refreshSessionCallCount == 1)
  }

  @Test func purchase_doesNotRefreshSession_onFailure() async {
    let (sut, subscriptionSpy, authSpy) = makeSUT()
    subscriptionSpy.purchaseResult = .failure(DomainError.purchaseFailed("declined"))

    await sut.purchase(productId: "uk.co.towncrier.personal.monthly")

    #expect(authSpy.refreshSessionCallCount == 0)
  }

  @Test func purchase_doesNotRefreshSession_onCancellation() async {
    let (sut, subscriptionSpy, authSpy) = makeSUT()
    subscriptionSpy.purchaseResult = .failure(DomainError.purchaseCancelled)

    await sut.purchase(productId: "uk.co.towncrier.personal.monthly")

    #expect(authSpy.refreshSessionCallCount == 0)
  }

  @Test func purchase_succeedsEvenWhenTokenRefreshFails() async {
    let (sut, subscriptionSpy, authSpy) = makeSUT()
    subscriptionSpy.purchaseResult = .success(.personalActive)
    authSpy.refreshSessionResult = .failure(DomainError.sessionExpired)

    await sut.purchase(productId: "uk.co.towncrier.personal.monthly")

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
}
